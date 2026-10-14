package httpjws

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"poc-spire/internal/identity"
	"poc-spire/internal/testpki"
)

func signTransfer(t *testing.T, now time.Time) (string, *testpki.CA) {
	t.Helper()
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	svid, err := ca.SVID(identity.Trusted, now)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"request_id": "transfer-1", "from": "demo-a", "to": "demo-b", "amount_minor": 100, "currency": "USD"})
	compact, err := Sign(svid, "POST", "/transfer", "transfer-1", body, now)
	if err != nil {
		t.Fatal(err)
	}
	return compact, ca
}

func TestTrustedPayloadVerifies(t *testing.T) {
	now := time.Now()
	compact, ca := signTransfer(t, now)
	verified, fail := Verify(compact, ca.Bundle(), now)
	if fail != nil || verified.Signer != identity.Trusted || string(verified.Body) == "null" {
		t.Fatal(fail, verified)
	}
}

func TestTamperedPayloadAndHeader(t *testing.T) {
	now := time.Now()
	compact, ca := signTransfer(t, now)
	parts := strings.Split(compact, ".")
	payload, _ := segment(parts[1], MaxPayload)
	payload[len(payload)-4] ^= 1
	parts[1] = base64.RawURLEncoding.EncodeToString(payload)
	if _, fail := Verify(strings.Join(parts, "."), ca.Bundle(), now); fail == nil || fail.Status != 401 || fail.Reason != "invalid_signature" {
		t.Fatal(fail)
	}
	header, _ := segment(parts[0], MaxHeader)
	header[0] ^= 1
	parts = strings.Split(compact, ".")
	parts[0] = base64.RawURLEncoding.EncodeToString(header)
	if _, fail := Verify(strings.Join(parts, "."), ca.Bundle(), now); fail == nil || fail.Status != 401 && fail.Status != 400 {
		t.Fatal(fail)
	}
}

func TestWrongSignatureBytes(t *testing.T) {
	now := time.Now()
	compact, ca := signTransfer(t, now)
	parts := strings.Split(compact, ".")
	sig, _ := segment(parts[2], 64)
	sig[0] ^= 0xff
	parts[2] = base64.RawURLEncoding.EncodeToString(sig)
	if _, fail := Verify(strings.Join(parts, "."), ca.Bundle(), now); fail == nil || fail.Status != 401 || fail.Reason != "invalid_signature" {
		t.Fatal(fail)
	}
}

func TestWrongIDStillVerifies(t *testing.T) {
	now := time.Now()
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	svid, err := ca.SVID(identity.Observer, now)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"request_id": "transfer-1", "from": "demo-a", "to": "demo-b", "amount_minor": 100, "currency": "USD"})
	compact, err := Sign(svid, "POST", "/transfer", "transfer-1", body, now)
	if err != nil {
		t.Fatal(err)
	}
	verified, fail := Verify(compact, ca.Bundle(), now)
	if fail != nil || verified.Signer != identity.Observer {
		t.Fatal(fail, verified)
	}
}

func TestObsoleteCertificateEnvelopeAndPreviousSVID(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	current, err := ca.SVIDBetween(identity.Trusted, now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	previous, err := ca.SVIDBetween(identity.Trusted, now.Add(-2*time.Minute), now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	body := json.RawMessage(`{"request_id":"transfer-1","from":"demo-a","to":"demo-b","amount_minor":100,"currency":"USD"}`)
	compact, err := Sign(previous, "POST", "/transfer", "transfer-1", body, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, fail := Verify(compact, ca.Bundle(), now); fail != nil {
		t.Fatal("previous SVID", fail)
	}
	_ = current
	expired, err := ca.SVIDBetween(identity.Trusted, now.Add(-2*time.Hour), now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	expiredCompact, err := seal(expired, "POST", "/transfer", "transfer-1", body, now.Add(-2*time.Minute).Unix(), now.Add(-2*time.Minute).Unix()+30)
	if err != nil {
		t.Fatal(err)
	}
	if _, fail := Verify(expiredCompact, ca.Bundle(), now); fail == nil || fail.Status != 403 || fail.Reason != "credential_not_current" {
		t.Fatal(fail)
	}
	future, err := ca.SVIDBetween(identity.Trusted, now.Add(6*time.Second), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	futureCompact, err := seal(future, "POST", "/transfer", "transfer-1", body, now.Unix(), now.Unix()+30)
	if err != nil {
		t.Fatal(err)
	}
	if _, fail := Verify(futureCompact, ca.Bundle(), now); fail == nil || fail.Status != 403 {
		t.Fatal(fail)
	}
	stale, err := seal(current, "POST", "/transfer", "transfer-1", body, now.Add(-31*time.Second).Unix(), now.Add(-time.Second).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if _, fail := Verify(stale, ca.Bundle(), now); fail == nil || fail.Status != 403 || fail.Reason != "credential_not_current" {
		t.Fatal("elapsed", fail)
	}
	wide, err := seal(current, "POST", "/transfer", "transfer-1", body, now.Unix(), now.Unix()+31)
	if err != nil {
		t.Fatal(err)
	}
	if _, fail := Verify(wide, ca.Bundle(), now); fail == nil || fail.Status != 403 {
		t.Fatal("wide", fail)
	}
}

func TestClockBoundary(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	svid, err := ca.SVIDBetween(identity.Trusted, now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	body := json.RawMessage(`{"request_id":"transfer-1","from":"demo-a","to":"demo-b","amount_minor":100,"currency":"USD"}`)
	allowed, err := seal(svid, "POST", "/transfer", "transfer-1", body, now.Unix()+5, now.Unix()+20)
	if err != nil {
		t.Fatal(err)
	}
	if _, fail := Verify(allowed, ca.Bundle(), now); fail != nil {
		t.Fatal(fail)
	}
	ahead, err := seal(svid, "POST", "/transfer", "transfer-1", body, now.Unix()+6, now.Unix()+20)
	if err != nil {
		t.Fatal(err)
	}
	if _, fail := Verify(ahead, ca.Bundle(), now); fail == nil || fail.Status != 403 {
		t.Fatal(fail)
	}
}

func TestBadSignatureBeatsExpiredCertificate(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	expired, err := ca.SVIDBetween(identity.Trusted, now.Add(-2*time.Hour), now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	body := json.RawMessage(`{"request_id":"transfer-1","from":"demo-a","to":"demo-b","amount_minor":100,"currency":"USD"}`)
	compact, err := seal(expired, "POST", "/transfer", "transfer-1", body, now.Add(-2*time.Minute).Unix(), now.Add(-90*time.Second).Unix())
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(compact, ".")
	sig, _ := segment(parts[2], 64)
	sig[1] ^= 0xff
	parts[2] = base64.RawURLEncoding.EncodeToString(sig)
	if _, fail := Verify(strings.Join(parts, "."), ca.Bundle(), now); fail == nil || fail.Status != 401 || fail.Reason != "invalid_signature" {
		t.Fatal(fail)
	}
}

func TestForgedChainAndMissingSignature(t *testing.T) {
	now := time.Now()
	compact, ca := signTransfer(t, now)
	other, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	if _, fail := Verify(compact, other.Bundle(), now); fail == nil || fail.Status != 401 || fail.Reason != "untrusted_svid" {
		t.Fatal(fail)
	}
	if _, fail := Verify("", ca.Bundle(), now); fail == nil || fail.Status != 401 {
		t.Fatal(fail)
	}
	if _, fail := Verify(`{"request_id":"transfer-1"}`, ca.Bundle(), now); fail == nil || fail.Status != 401 || fail.Reason != "invalid_signature" {
		t.Fatal("unsigned json", fail)
	}
	parts := strings.Split(compact, ".")
	header, _ := segment(parts[0], MaxHeader)
	var fields map[string]any
	json.Unmarshal(header, &fields)
	fields["alg"] = "none"
	raw, _ := json.Marshal(fields)
	parts[0] = base64.RawURLEncoding.EncodeToString(raw)
	parts[2] = base64.RawURLEncoding.EncodeToString(make([]byte, 64))
	if _, fail := Verify(strings.Join(parts, "."), ca.Bundle(), now); fail == nil || fail.Status != 401 {
		t.Fatal(fail)
	}
}
