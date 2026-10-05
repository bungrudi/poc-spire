package money

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
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
