package money

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

// These unit cases test route policy after authentication. Real trust and
// certificate rejection are exercised separately by the SPIRE integration suite.
func TestRoutePolicyBeforeParsingOrLedger(t *testing.T) {
	book := ledger.New(10)
	h := &handler{ledger: book, audit: io.Discard}
	for _, id := range []string{"", identity.Observer.String(), "spiffe://poc.internal/test/wrong-id", identity.Trusted.String() + "/extra"} {
		r := httptest.NewRequest("POST", "/transfer", strings.NewReader(strings.Repeat("x", 10000)))
		r.Header.Set("X-Spiffe-Id", identity.Trusted.String())
		if id != "" {
			u, _ := url.Parse(id)
			r.TLS = &tls.ConnectionState{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{{URIs: []*url.URL{u}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute)}}}
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Error(id, w.Code)
		}
	}
	if s := book.Snapshot(); s.Version != 0 {
		t.Fatal(s)
	}
	if len(book.Snapshot().Balances) != 2 {
		t.Fatal("balances changed")
	}
}
func TestBoundedInputAndObserverPolicy(t *testing.T) {
	h := &handler{ledger: ledger.New(10), audit: io.Discard}
	for _, body := range []string{strings.Repeat(" ", 4097) + `{}`, `{"request_id":"a","unexpected":1}`, `{} {}`} {
		r := httptest.NewRequest("POST", "/transfer", strings.NewReader(body))
		u, _ := url.Parse(identity.Trusted.String())
		r.TLS = &tls.ConnectionState{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{{URIs: []*url.URL{u}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute)}}}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Error(w.Code)
		}
	}
	if !permitted("GET", "/balances", identity.Observer.String()) || permitted("POST", "/transfer", identity.Observer.String()) || permitted("GET", "/balances", identity.Trusted.String()) {
		t.Fatal("role confusion")
	}
}

// Model a handshake that completed while valid, followed by expiry during input.
type expireOnRead struct {
	cert *x509.Certificate
	body io.Reader
}

func (r *expireOnRead) Read(b []byte) (int, error) {
	r.cert.NotAfter = time.Now().Add(-time.Second)
	return r.body.Read(b)
}
func TestCredentialExpiryCannotReachLedger(t *testing.T) {
	for _, mode := range []string{"expired", "future", "expires-during-body"} {
		t.Run(mode, func(t *testing.T) {
			book := ledger.New(10)
			h := &handler{ledger: book, audit: io.Discard}
			u, _ := url.Parse(identity.Trusted.String())
			cert := &x509.Certificate{URIs: []*url.URL{u}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute)}
			body := io.Reader(strings.NewReader(`{"request_id":"expiry-test","from":"demo-a","to":"demo-b","amount_minor":100,"currency":"USD"}`))
			switch mode {
			case "expired":
				cert.NotAfter = time.Now().Add(-time.Second)
			case "future":
				cert.NotBefore = time.Now().Add(time.Minute)
			case "expires-during-body":
				body = &expireOnRead{cert: cert, body: body}
			}
			r := httptest.NewRequest("POST", "/transfer", body)
			r.TLS = &tls.ConnectionState{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{cert}}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 403 || book.Snapshot().Version != 0 {
				t.Fatalf("expired credential reached business logic: status=%d version=%d", w.Code, book.Snapshot().Version)
			}
		})
	}
}

// Capacity must remain safe at the HTTP/authorization boundary, not just in the
// ledger package. Synthetic valid peer data isolates this handler behavior;
// real-SPIRE suites separately verify TLS authentication and exact route policy.
func TestHTTPHistoryCapacityCannotForgetOrAuthorizeKeys(t *testing.T) {
	book := ledger.New(2)
	h := &handler{ledger: book, audit: io.Discard}
	send := func(peer, id string) (int, ledger.Result) {
		body, _ := json.Marshal(ledger.Fixed(id))
		r := httptest.NewRequest("POST", "/transfer", bytes.NewReader(body))
		u, _ := url.Parse(peer)
		r.TLS = &tls.ConnectionState{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{{URIs: []*url.URL{u}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute)}}}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var result ledger.Result
		json.Unmarshal(w.Body.Bytes(), &result)
		return w.Code, result
	}
	if status, _ := send(identity.Observer.String(), "denied"); status != 403 {
		t.Fatal(status)
	}
	firstStatus, first := send(identity.Trusted.String(), "first")
	secondStatus, _ := send(identity.Trusted.String(), "second")
	if firstStatus != 200 || secondStatus != 200 {
		t.Fatal("Denied identity consumed history capacity")
	}
	if status, result := send(identity.Trusted.String(), "third"); status != 503 || result.Error != "history_capacity_exhausted" {
		t.Fatal(status, result)
	}
	status, replay := send(identity.Trusted.String(), "first")
	if status != 200 || !replay.Replay || replay.TransferID != first.TransferID || replay.Version != first.Version || book.Snapshot().Version != 2 {
		t.Fatal("Capacity changed or forgot a committed key", status, replay)
	}
	snapshot := book.Snapshot()
	if snapshot.Balances["demo-a"]+snapshot.Balances["demo-b"] != 200000 {
		t.Fatal("Balance conservation failed")
	}
}

func TestRequestObservationUsesConnectionMetadataWithoutSensitiveInputs(t *testing.T) {
	var audit bytes.Buffer
	h := &handler{ledger: ledger.New(10), audit: &audit}
	r := httptest.NewRequest("POST", "/transfer?token=QUERY_SECRET", strings.NewReader("JWS_BODY_SECRET"))
	r.Header.Set("Authorization", "TOKEN_SECRET")
	r.Header.Set("Cookie", "COOKIE_SECRET")
	r.Header.Set("X-Forwarded-For", "FORGED_REMOTE")
	r.RemoteAddr = "127.0.0.6:41013"
	r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8443}))
	uri, _ := url.Parse(identity.Trusted.String())
	r.TLS = &tls.ConnectionState{Version: tls.VersionTLS13, HandshakeComplete: true, PeerCertificates: []*x509.Certificate{{Raw: []byte("SVID_CERT_SECRET"), URIs: []*url.URL{uri}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute)}}}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var event map[string]any
	if err := json.Unmarshal(audit.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if w.Code != 400 || event["status"] != float64(400) || event["verified_peer"] != identity.Trusted.String() || event["remote"] != "127.0.0.6:41013" || event["local"] != "127.0.0.1:8443" || event["tls_version"] != "TLS 1.3" || event["tls_session_resumed"] != false {
		t.Fatal(event, w.Code)
	}
	stamp, err := time.Parse(time.RFC3339Nano, event["at_utc"].(string))
	if err != nil {
		t.Fatal(err)
	}
	_, offset := stamp.Zone()
	if offset != 0 {
		t.Fatal("observation is not UTC")
	}
	for _, secret := range []string{"QUERY_SECRET", "JWS_BODY_SECRET", "TOKEN_SECRET", "COOKIE_SECRET", "SVID_CERT_SECRET", "FORGED_REMOTE"} {
		if strings.Contains(audit.String(), secret) {
			t.Fatal("sensitive/arbitrary input reached audit")
		}
	}
	if h.ledger.Snapshot().Version != 0 {
		t.Fatal("observation changed rejection behavior")
	}
}
