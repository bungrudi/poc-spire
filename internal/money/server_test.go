package money

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"net"
	"net/http"
	"net/http/httptest"
	"poc-spire/internal/httpjws"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
	"poc-spire/internal/testpki"
	"strings"
	"testing"
	"time"
)

func logicalCall(t *testing.T, h *signedHandler, ca *testpki.CA, peer, method, path, id string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	wire := ""
	if peer != "" {
		svid, err := ca.SVID(spiffeid.RequireFromString(peer), h.clock())
		if err != nil {
			t.Fatal(err)
		}
		if len(body) == 0 {
			body = []byte("null")
		}
		wire, err = httpjws.Sign(svid, method, strings.Split(path, "?")[0], id, body, h.clock())
		if err != nil {
			t.Fatal(err)
		}
	}
	return signedCall(t, h, method, path, wire)
}
func responseBody(t *testing.T, w *httptest.ResponseRecorder, ca *testpki.CA, now time.Time) []byte {
	t.Helper()
	v, fail := httpjws.Verify(w.Body.String(), ca.Bundle(), now)
	if fail != nil || v.Signer != identity.Money {
		t.Fatal("unverified response", fail)
	}
	return v.Body
}
func TestRoutePolicyBeforeParsingOrLedger(t *testing.T) {
	h, ca, book := signedFixture(t, time.Now())
	for _, id := range []string{"", identity.Observer.String(), "spiffe://poc.internal/test/wrong-id", identity.Trusted.String() + "/extra"} {
		w := logicalCall(t, h, ca, id, "POST", "/transfer", "one", []byte(`{"unexpected":true}`))
		expected := 403
		if id == "" {
			expected = 401
		}
		if w.Code != expected {
			t.Fatal(id, w.Code)
		}
	}
	if book.Snapshot().Version != 0 {
		t.Fatal("unauthorized effect")
	}
}
func TestBoundedInputAndObserverPolicy(t *testing.T) {
	h, ca, book := signedFixture(t, time.Now())
	for _, body := range []string{`{"request_id":"a","unexpected":1}`, `{}`, `{"request_id":"a","from":"demo-a","to":"demo-b","amount_minor":100,"currency":"USD","extra":1}`} {
		if w := logicalCall(t, h, ca, identity.Trusted.String(), "POST", "/transfer", "a", []byte(body)); w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	if w := signedCall(t, h, "POST", "/transfer", strings.Repeat("x", httpjws.MaxValue+1)); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if !permitted("GET", "/balances", identity.Observer.String()) || permitted("POST", "/transfer", identity.Observer.String()) || permitted("GET", "/balances", identity.Trusted.String()) || book.Snapshot().Version != 0 {
		t.Fatal("policy or effects")
	}
}
func TestHTTPHistoryCapacityCannotForgetOrAuthorizeKeys(t *testing.T) {
	now := time.Now()
	h, ca, book := signedFixture(t, now)
	book2 := ledger.New(2)
	h.ledger = book2
	book = book2
	send := func(peer, id string) (int, ledger.Result) {
		w := logicalCall(t, h, ca, peer, "POST", "/transfer", id, mustBody(id))
		var result ledger.Result
		json.Unmarshal(responseBody(t, w, ca, now), &result)
		return w.Code, result
	}
	if status, _ := send(identity.Observer.String(), "denied"); status != 403 {
		t.Fatal(status)
	}
	a, first := send(identity.Trusted.String(), "first")
	b, _ := send(identity.Trusted.String(), "second")
	if a != 200 || b != 200 {
		t.Fatal(a, b)
	}
	if status, result := send(identity.Trusted.String(), "third"); status != 503 || result.Error != "history_capacity_exhausted" {
		t.Fatal(status, result)
	}
	status, replay := send(identity.Trusted.String(), "first")
	if status != 200 || !replay.Replay || replay.TransferID != first.TransferID || book.Snapshot().Version != 2 {
		t.Fatal(status, replay)
	}
}
func TestRequestObservationUsesConnectionMetadataWithoutSensitiveInputs(t *testing.T) {
	now := time.Now()
	h, ca, book := signedFixture(t, now)
	var audit bytes.Buffer
	h.audit = &audit
	svid, _ := ca.SVID(identity.Observer, now)
	wire, err := httpjws.Sign(svid, "POST", "/transfer", "denied", []byte(`{"secret":"JWS_BODY_SECRET"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/transfer?token=QUERY_SECRET", strings.NewReader(wire))
	r.Header.Set("Authorization", "TOKEN_SECRET")
	r.Header.Set("Cookie", "COOKIE_SECRET")
	r.Header.Set("X-Forwarded-For", "FORGED_REMOTE")
	r.RemoteAddr = "127.0.0.6:41013"
	r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8443}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var event map[string]any
	if json.Unmarshal(audit.Bytes(), &event) != nil {
		t.Fatal(audit.String())
	}
	if w.Code != 403 || event["verified_peer"] != identity.Observer.String() || event["transport"] != "signed-http" || event["local"] != "127.0.0.1:8443" || event["remote"] != "127.0.0.6:41013" {
		t.Fatal(event, w.Code)
	}
	for _, secret := range []string{"QUERY_SECRET", "JWS_BODY_SECRET", "TOKEN_SECRET", "COOKIE_SECRET", "FORGED_REMOTE", wire} {
		if strings.Contains(audit.String(), secret) {
			t.Fatal("secret leaked")
		}
	}
	if book.Snapshot().Version != 0 {
		t.Fatal("audit changed effects")
	}
}
