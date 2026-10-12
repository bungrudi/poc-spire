package kafkacmd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

var testNow = time.Unix(1791780000, 0)

func fixture(t *testing.T, id spiffeid.ID, curve elliptic.Curve) (*x509svid.SVID, *x509bundle.Bundle) {
	t.Helper()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rootTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, NotBefore: testNow.Add(-time.Hour), NotAfter: testNow.Add(time.Hour)}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	root, _ := x509.ParseCertificate(rootDER)
	key, _ := ecdsa.GenerateKey(curve, rand.Reader)
	uri, _ := url.Parse(id.String())
	leafTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), URIs: []*url.URL{uri}, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, NotBefore: testNow.Add(-time.Minute), NotAfter: testNow.Add(time.Minute)}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, root, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(leafDER)
	return &x509svid.SVID{ID: id, Certificates: []*x509.Certificate{leaf}, PrivateKey: key}, x509bundle.FromX509Authorities(id.TrustDomain(), []*x509.Certificate{root})
}
func testPolicy(t *testing.T) Policy {
	t.Helper()
	p, err := NewPolicy("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func seal(t *testing.T, svid *x509svid.SVID, p Policy, id string) Record {
	t.Helper()
	r, fail := Seal(svid, p.Context(testNow), ledger.Fixed(id), testNow)
	if fail != nil {
		t.Fatal(fail)
	}
	return r
}

// rawSeal is test-only and admits deliberately invalid profile fields.
func rawSeal(t *testing.T, svid *x509svid.SVID, payload []byte, headers map[jose.HeaderKey]any) []byte {
	t.Helper()
	chain := []string{}
	for _, cert := range svid.Certificates {
		chain = append(chain, base64.StdEncoding.EncodeToString(cert.Raw))
	}
	opts := (&jose.SignerOptions{}).WithType(jose.ContentType(Type)).WithHeader(jose.HeaderKey("x5c"), chain)
	for k, v := range headers {
		opts.WithHeader(k, v)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: svid.PrivateKey}, opts)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	compact, err := signed.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return []byte(compact)
}
func TestTrustedSignatureAndExactRecordBindings(t *testing.T) {
	svid, bundle := fixture(t, identity.Trusted, elliptic.P256())
	policy := testPolicy(t)
	record := seal(t, svid, policy, "one")
	verified, fail := Verify(record, policy, bundle, testNow)
	if fail != nil || verified.Producer != identity.Trusted.String() {
		t.Fatal("trusted failed", fail)
	}
	if verified.Message.Transfer != ledger.Fixed("one") {
		t.Fatal("typed transfer changed")
	}
	for _, name := range []string{"topic", "key", "headers", "signature", "payload-byte"} {
		t.Run(name, func(t *testing.T) {
			r := record
			r.Key = append([]byte{}, record.Key...)
			r.Value = append([]byte{}, record.Value...)
			switch name {
			case "topic":
				r.Topic += ".wrong"
			case "key":
				r.Key = []byte("other")
			case "headers":
				r.Headers = []Header{{"X-Spiffe-Id", []byte(identity.Trusted.String())}}
			case "signature":
				parts := strings.Split(string(r.Value), ".")
				sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
				sig[0] ^= 1
				parts[2] = base64.RawURLEncoding.EncodeToString(sig)
				r.Value = []byte(strings.Join(parts, "."))
			case "payload-byte":
				parts := strings.Split(string(r.Value), ".")
				payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
				parts[1] = base64.RawURLEncoding.EncodeToString(append([]byte(" "), payload...))
				r.Value = []byte(strings.Join(parts, "."))
			}
			v, fail := Verify(r, policy, bundle, testNow)
			if v != nil || fail == nil || fail.Transient {
				t.Fatal("altered record accepted", name)
			}
			if (name == "signature" || name == "payload-byte") && fail.VerifiedID != "" {
				t.Fatal("invalid signature attributed to claimed producer")
			}
		})
	}
}
func TestWrongIdentityAndRootAreDifferentFromMissingTrust(t *testing.T) {
	policy := testPolicy(t)
	trusted, _ := fixture(t, identity.Trusted, elliptic.P256())
	fake := seal(t, trusted, policy, "fake")
	_, otherBundle := fixture(t, identity.Trusted, elliptic.P256())
	if _, fail := Verify(fake, policy, otherBundle, testNow); fail == nil || fail.Reason != "untrusted_svid" || fail.Transient || fail.VerifiedID != "" {
		t.Fatal(fail)
	}
	if _, fail := Verify(fake, policy, nil, testNow); fail == nil || !fail.Transient {
		t.Fatal("missing trust terminal")
	}
	// A valid differently identified leaf is signed with a profile-compatible key;
	// its own independent bundle establishes authentication before exact-ID denial.
	observer, observerBundle := fixture(t, identity.Observer, elliptic.P256())
	msg := Message{policy.Context(testNow), ledger.Fixed("wrong-id"), testNow.Unix(), testNow.Unix() + 30}
	payload, _ := json.Marshal(msg)
	r := Record{Topic: policy.Topic, Key: []byte("wrong-id"), Value: rawSeal(t, observer, payload, nil)}
	if _, fail := Verify(r, policy, observerBundle, testNow); fail == nil || fail.Reason != "unauthorized_signer" || fail.VerifiedID != identity.Observer.String() {
		t.Fatal(fail)
	}
}
func TestStrictJSONAmbiguityUnicodeAndDepth(t *testing.T) {
	invalid := []string{`{"a":1,"a":2}`, `{"a":1,"\u0061":2}`, `{"a":null}`, `{"a":1.0}`, `{"a":1e0}`, `{"a":"\ud800"}`, `{"a":"\udc00"}`, `{"a":"\ud800x"}`, `{"a":9223372036854775808}`, `{"a":{"b":{"c":{"d":1}}}}`, `{"a":1} {"b":2}`, "{\"a\":\"\xff\"}"}
	for _, raw := range invalid {
		if _, err := StrictJSON([]byte(raw)); err == nil {
			t.Errorf("accepted ambiguous input: %q", raw)
		}
	}
	for _, raw := range []string{`{"a":1}`, `{"a":"\ud83d\ude00"}`, `{"a":"\\ud800"}`, `{"a":{"b":{"c":1}}}`} {
		if _, err := StrictJSON([]byte(raw)); err != nil {
			t.Errorf("valid input rejected %s: %v", raw, err)
		}
	}
}
func TestProfileLimitsAndAlgorithmHeaders(t *testing.T) {
	svid, bundle := fixture(t, identity.Trusted, elliptic.P256())
	p := testPolicy(t)
	record := seal(t, svid, p, "limits")
	parsed, _ := Parse(record)
	payload, _ := json.Marshal(parsed.Message)
	for _, header := range []string{"crit", "jku", "jwk", "kid", "x5u", "b64", "zip"} {
		t.Run(header, func(t *testing.T) {
			r := record
			value := any("untrusted")
			if header == "b64" {
				value = true
			}
			r.Value = rawSeal(t, svid, payload, map[jose.HeaderKey]any{jose.HeaderKey(header): value})
			if _, f := Verify(r, p, bundle, testNow); f == nil {
				t.Fatal("unexpected header accepted")
			}
		})
	}
	for _, value := range []string{"", "e30.e30.", "e30=.e30.AA", string(record.Value) + ".", strings.Repeat("A", MaxValue+1)} {
		r := record
		r.Value = []byte(value)
		if _, f := Parse(r); f == nil {
			t.Fatal("bad compact accepted")
		}
	}
	for _, alg := range []string{"none", "HS256", "ES384"} {
		r := record
		parts := strings.Split(string(r.Value), ".")
		header, _ := base64.RawURLEncoding.DecodeString(parts[0])
		header = []byte(strings.Replace(string(header), "ES256", alg, 1))
		parts[0] = base64.RawURLEncoding.EncodeToString(header)
		r.Value = []byte(strings.Join(parts, "."))
		if _, f := Verify(r, p, bundle, testNow); f == nil {
			t.Fatal("alternate algorithm accepted")
		}
	}
	wrongCurve, _ := fixture(t, identity.Trusted, elliptic.P384())
	if _, f := Seal(wrongCurve, p.Context(testNow), ledger.Fixed("curve"), testNow); f == nil {
		t.Fatal("P384 signing allowed")
	}
	svid.PrivateKey = wrongCurve.PrivateKey
	if _, f := Seal(svid, p.Context(testNow), ledger.Fixed("mismatch"), testNow); f == nil {
		t.Fatal("mismatched key allowed")
	}
}
func TestFixedMenuHasNoUnsignedFallbackOrSignatureAttribution(t *testing.T) {
	svid, bundle := fixture(t, identity.Trusted, elliptic.P256())
	p := testPolicy(t)
	for _, scenario := range []string{"kafka-trusted", "kafka-unidentified", "kafka-tampered"} {
		r, f := FixedRecord(svid, p.Context(testNow), "fixed", scenario, testNow)
		if f != nil {
			t.Fatal(f)
		}
		v, f := Verify(r, p, bundle, testNow)
		if scenario == "kafka-trusted" {
			if f != nil || v == nil {
				t.Fatal(f)
			}
		} else {
			if f == nil || v != nil || f.VerifiedID != "" {
				t.Fatal("negative probe accepted or attributed", f)
			}
			if scenario == "kafka-tampered" && f.Reason != "invalid_signature" {
				t.Fatal(f)
			}
		}
	}
	if _, f := FixedRecord(svid, p.Context(testNow), "fixed", "arbitrary", testNow); f == nil {
		t.Fatal("arbitrary scenario allowed")
	}
	if _, f := Seal(nil, p.Context(testNow), ledger.Fixed("fixed"), testNow); f == nil || !f.Transient {
		t.Fatal("no signer fell back")
	}
}
func TestFreshnessClockBudgetAndAssignmentBoundaries(t *testing.T) {
	svid, bundle := fixture(t, identity.Trusted, elliptic.P256())
	p := testPolicy(t)
	record := seal(t, svid, p, "time")
	parsed, _ := Parse(record)
	for _, delta := range []int64{5, 6} {
		m := parsed.Message
		m.IssuedAt += delta
		m.ExpiresAt += delta
		payload, _ := json.Marshal(m)
		r := record
		r.Value = rawSeal(t, svid, payload, nil)
		_, f := Verify(r, p, bundle, testNow)
		if (delta == 5) != (f == nil) {
			t.Fatal("future tolerance", delta, f)
		}
	}
	v, f := Verify(record, p, bundle, testNow)
	if f != nil {
		t.Fatal(f)
	}
	if f := v.Guard(testNow.Add(2*time.Second), true); f != nil {
		t.Fatal("budget exact boundary", f)
	}
	if f := v.Guard(testNow.Add(2*time.Second+time.Nanosecond), true); f == nil || !f.Transient {
		t.Fatal("budget not transient")
	}
	if f := v.Guard(testNow, false); f == nil || !f.Transient {
		t.Fatal("assignment not transient")
	}
	// Freshness failure at the lock boundary must be terminal while still inside
	// the local verification budget. Inject an envelope with one second remaining.
	m := parsed.Message
	m.IssuedAt = testNow.Unix() - 29
	m.ExpiresAt = testNow.Unix() + 1
	m.Context.CreatedAt = testNow.Unix() - 29
	m.Context.Deadline = m.Context.CreatedAt + 120
	payload, _ := json.Marshal(m)
	r := record
	r.Value = rawSeal(t, svid, payload, nil)
	v, f = Verify(r, p, bundle, testNow)
	if f != nil {
		t.Fatal(f)
	}
	if f := v.Guard(testNow.Add(time.Second), true); f == nil || f.Transient || f.Reason != "message_expired" {
		t.Fatal("expiry not terminal", f)
	}
	if _, f := Verify(record, p, bundle, svid.Certificates[0].NotAfter); f == nil || f.Transient {
		t.Fatal("expired chain accepted")
	}
	near := *svid
	leaf := *svid.Certificates[0]
	leaf.NotAfter = testNow.Add(10 * time.Second)
	near.Certificates = []*x509.Certificate{&leaf}
	if _, f := Seal(&near, p.Context(testNow), ledger.Fixed("near"), testNow); f == nil {
		t.Fatal("<=5 seconds publish allowed")
	}
}
func TestDigestNullEmptyFieldsAndHeaderOrder(t *testing.T) {
	base := Record{Topic: "topic"}
	variants := []Record{base, {Topic: "topic", Key: []byte{}}, {Topic: "topic", Value: []byte{}}, {Topic: "topic", Headers: []Header{{"a", nil}, {"b", []byte("v")}}}, {Topic: "topic", Headers: []Header{{"b", []byte("v")}, {"a", nil}}}, {Topic: "topic-other"}}
	seen := map[string]bool{}
	for _, r := range variants {
		d := Digest(r)
		if !Hex64.MatchString(d) || seen[d] {
			t.Fatal("digest framing collision")
		}
		seen[d] = true
	}
}

// The public test key and vector are RFC 7515 Appendix A.3. They are codec
// fixtures, not an application credential or a valid SPIFFE command.
func TestRFC7515AppendixA3ES256(t *testing.T) {
	var jwk jose.JSONWebKey
	if err := json.Unmarshal([]byte(`{"kty":"EC","crv":"P-256","x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU","y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"}`), &jwk); err != nil {
		t.Fatal(err)
	}
	compact := "eyJhbGciOiJFUzI1NiJ9.eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ.DtEhU3ljbEg8L38VWAfUAqOyKAM6-Xx-F4GawxaepmXFCgfTjDxw5djxLa8ISlSApmWQxfKTUJqPP3-Kg6NU1Q"
	signed, err := jose.ParseSignedCompact(compact, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signed.Verify(jwk.Key); err != nil {
		t.Fatal("RFC vector verification failed", err)
	}
	key, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if _, err := signed.Verify(&key.PublicKey); err == nil {
		t.Fatal("curve confusion accepted")
	}
	parts := strings.Split(compact, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	parts[1] = base64.RawURLEncoding.EncodeToString(append(payload, ' '))
	altered, _ := jose.ParseSignedCompact(strings.Join(parts, "."), []jose.SignatureAlgorithm{jose.ES256})
	if _, err := altered.Verify(jwk.Key); err == nil {
		t.Fatal("altered exact bytes accepted")
	}
	if _, f := Parse(Record{Topic: "topic", Key: []byte("rfc"), Value: []byte(compact)}); f == nil {
		t.Fatal("codec vector confused with valid application profile")
	}
}
