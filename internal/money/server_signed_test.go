package money

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"poc-spire/internal/httpjws"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
	"poc-spire/internal/testpki"
)

func signedFixture(t *testing.T, now time.Time) (*signedHandler, *testpki.CA, *ledger.Ledger) {
	t.Helper()
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	moneySVID, err := ca.SVID(identity.Money, now)
	if err != nil {
		t.Fatal(err)
	}
	book := ledger.New(10)
	h := &signedHandler{ledger: book, audit: io.Discard, signer: testpki.Source{SVID: moneySVID}, bundle: ca.Bundle(), now: func() time.Time { return now }}
	return h, ca, book
}

func signedCall(t *testing.T, h *signedHandler, method, path, compact string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(compact))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestSignedTransferAndRead(t *testing.T) {
	now := time.Now()
	h, ca, book := signedFixture(t, now)
	trusted, err := ca.SVID(identity.Trusted, now)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(ledger.Fixed("transfer-1"))
	compact, err := httpjws.Sign(trusted, "POST", "/transfer", "transfer-1", body, now)
	if err != nil {
		t.Fatal(err)
	}
	w := signedCall(t, h, "POST", "/transfer", compact)
	if w.Code != 200 || book.Snapshot().Version != 1 {
		t.Fatal(w.Code, book.Snapshot())
	}
	verified, fail := httpjws.Verify(w.Body.String(), ca.Bundle(), now)
	if fail != nil || verified.Signer != identity.Money || verified.RequestID != "transfer-1" {
		t.Fatal(fail)
	}
	again := signedCall(t, h, "POST", "/transfer", compact)
	if again.Code != 200 || book.Snapshot().Version != 1 {
		t.Fatal("retry", again.Code, book.Snapshot().Version)
	}
	ui, err := ca.SVID(identity.Observer, now)
	if err != nil {
		t.Fatal(err)
	}
	read, err := httpjws.Sign(ui, "GET", "/balances", "balance-1", json.RawMessage("null"), now)
	if err != nil {
		t.Fatal(err)
	}
	balances := signedCall(t, h, "GET", "/balances", read)
	if balances.Code != 200 {
		t.Fatal(balances.Code, balances.Body.String())
	}
	deniedBody, _ := json.Marshal(ledger.Fixed("transfer-2"))
	deniedCompact, err := httpjws.Sign(ui, "POST", "/transfer", "transfer-2", deniedBody, now)
	if err != nil {
		t.Fatal(err)
	}
	denied := signedCall(t, h, "POST", "/transfer", deniedCompact)
	if denied.Code != 403 || book.Snapshot().Version != 1 {
		t.Fatal(denied.Code, book.Snapshot().Version)
	}
}

func TestSignedNegativePayloads(t *testing.T) {
	now := time.Now()
	h, ca, book := signedFixture(t, now)
	trusted, err := ca.SVID(identity.Trusted, now)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(ledger.Fixed("transfer-1"))
	compact, err := httpjws.Sign(trusted, "POST", "/transfer", "transfer-1", body, now)
	if err != nil {
		t.Fatal(err)
	}
	if w := signedCall(t, h, "POST", "/transfer", ""); w.Code != 401 || book.Snapshot().Version != 0 {
		t.Fatal("missing", w.Code)
	}
	parts := strings.Split(compact, ".")
	payload, _ := base64.RawURLEncoding.Strict().DecodeString(parts[1])
	payload = bytesReplace(payload, []byte("100"), []byte("200"))
	parts[1] = base64.RawURLEncoding.EncodeToString(payload)
	if w := signedCall(t, h, "POST", "/transfer", strings.Join(parts, ".")); w.Code != 401 || book.Snapshot().Version != 0 {
		t.Fatal("tamper", w.Code)
	}
	parts = strings.Split(compact, ".")
	sig, _ := base64.RawURLEncoding.Strict().DecodeString(parts[2])
	sig[0] ^= 0xff
	parts[2] = base64.RawURLEncoding.EncodeToString(sig)
	if w := signedCall(t, h, "POST", "/transfer", strings.Join(parts, ".")); w.Code != 401 || book.Snapshot().Version != 0 {
		t.Fatal("wrong signature", w.Code)
	}
	other, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := other.SVID(identity.Trusted, now)
	if err != nil {
		t.Fatal(err)
	}
	forged, err := httpjws.Sign(foreign, "POST", "/transfer", "transfer-1", body, now)
	if err != nil {
		t.Fatal(err)
	}
	if w := signedCall(t, h, "POST", "/transfer", forged); w.Code != 401 || book.Snapshot().Version != 0 {
		t.Fatal("forged", w.Code)
	}
	replay, err := httpjws.Sign(trusted, "GET", "/balances", "transfer-1", json.RawMessage("null"), now)
	if err != nil {
		t.Fatal(err)
	}
	if w := signedCall(t, h, "POST", "/transfer", replay); w.Code != 401 || book.Snapshot().Version != 0 {
		t.Fatal("replay", w.Code)
	}
	staleNow := now.Add(31 * time.Second)
	h.now = func() time.Time { return staleNow }
	if w := signedCall(t, h, "POST", "/transfer", compact); w.Code != 403 || book.Snapshot().Version != 0 {
		t.Fatal("obsolete", w.Code, w.Body.String())
	}
	h.now = func() time.Time { return now }
	previous, err := ca.SVIDBetween(identity.Trusted, now.Add(-2*time.Minute), now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	previousCompact, err := httpjws.Sign(previous, "POST", "/transfer", "transfer-9", mustBody("transfer-9"), now)
	if err != nil {
		t.Fatal(err)
	}
	if w := signedCall(t, h, "POST", "/transfer", previousCompact); w.Code != 200 || book.Snapshot().Version != 1 {
		t.Fatal("previous", w.Code, book.Snapshot().Version)
	}
}

func mustBody(id string) []byte {
	raw, _ := json.Marshal(ledger.Fixed(id))
	return raw
}

func bytesReplace(raw, old, new []byte) []byte {
	return []byte(strings.Replace(string(raw), string(old), string(new), 1))
}
