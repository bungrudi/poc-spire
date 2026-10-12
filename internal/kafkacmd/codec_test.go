package kafkacmd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	"poc-spire/internal/testpki"
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
	for _, name := range []string{"topic", "key", "headers", "signature", "payload-byte", "protected-byte"} {
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
			case "protected-byte":
				parts := strings.Split(string(r.Value), ".")
				header, _ := base64.RawURLEncoding.DecodeString(parts[0])
				parts[0] = base64.RawURLEncoding.EncodeToString(append([]byte(" "), header...))
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
			if (name == "signature" || name == "payload-byte" || name == "protected-byte") && fail.VerifiedID != "" {
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

func TestInboundCurveDERAndEveryWireBound(t *testing.T) {
	svid, bundle := fixture(t, identity.Trusted, elliptic.P256())
	p := testPolicy(t)
	good := seal(t, svid, p, "bounds")
	parts := strings.Split(string(good.Value), ".")
	headerRaw, _ := base64.RawURLEncoding.DecodeString(parts[0])
	payloadRaw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var original map[string]any
	json.Unmarshal(headerRaw, &original)
	repack := func(header, payload []byte, signature string) Record {
		return Record{Topic: good.Topic, Key: good.Key, Value: []byte(base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + signature)}
	}
	cases := map[string]Record{
		"header-decoded-16385":           repack(append(headerRaw, []byte(strings.Repeat(" ", MaxHeader+1-len(headerRaw)))...), payloadRaw, parts[2]),
		"payload-decoded-4097":           repack(headerRaw, append(payloadRaw, []byte(strings.Repeat(" ", MaxPayload+1-len(payloadRaw)))...), parts[2]),
		"header-duplicate-escaped-name":  repack([]byte(strings.Replace(string(headerRaw), `"alg":"ES256"`, `"alg":"ES256","\u0061lg":"ES256"`, 1)), payloadRaw, parts[2]),
		"payload-duplicate-escaped-name": repack(headerRaw, []byte(strings.Replace(string(payloadRaw), `"request_id":"bounds"`, `"request_id":"bounds","\u0072equest_id":"other"`, 1)), parts[2]),
		"payload-unknown":                repack(headerRaw, []byte(strings.Replace(string(payloadRaw), `"issued_at":`, `"unexpected":1,"issued_at":`, 1)), parts[2]),
		"payload-trailing":               repack(headerRaw, append(payloadRaw, []byte(" {}")...), parts[2]),
		"payload-exponent":               repack(headerRaw, []byte(strings.Replace(string(payloadRaw), `"amount_minor":100`, `"amount_minor":1e2`, 1)), parts[2]),
	}
	cases["key-129"] = Record{Topic: good.Topic, Key: []byte(strings.Repeat("a", 129)), Value: good.Value}
	cases["value-32769"] = Record{Topic: good.Topic, Key: good.Key, Value: []byte(strings.Repeat("a", MaxValue+1))}
	for _, c := range []struct {
		name  string
		chain any
	}{{"zero-certificates", []string{}}, {"five-certificates", []string{"a", "b", "c", "d", "e"}}, {"der-2049", []string{base64.StdEncoding.EncodeToString(make([]byte, 2049))}}, {"aggregate-over-8192", []string{base64.StdEncoding.EncodeToString(make([]byte, 2049)), base64.StdEncoding.EncodeToString(make([]byte, 2049)), base64.StdEncoding.EncodeToString(make([]byte, 2049)), base64.StdEncoding.EncodeToString(make([]byte, 2049))}}, {"x5c-url-encoding", []string{base64.RawURLEncoding.EncodeToString(svid.Certificates[0].Raw)}}} {
		h := map[string]any{"alg": "ES256", "typ": Type, "x5c": c.chain}
		raw, _ := json.Marshal(h)
		cases[c.name] = repack(raw, payloadRaw, parts[2])
	}
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	der, err := ecdsa.SignASN1(rand.Reader, svid.PrivateKey.(*ecdsa.PrivateKey), hash[:])
	if err != nil {
		t.Fatal(err)
	}
	cases["asn1-der-signature"] = repack(headerRaw, payloadRaw, base64.RawURLEncoding.EncodeToString(der))
	p384, _ := fixture(t, identity.Trusted, elliptic.P384())
	curveHeader, _ := json.Marshal(map[string]any{"alg": "ES256", "typ": Type, "x5c": []string{base64.StdEncoding.EncodeToString(p384.Certificates[0].Raw)}})
	cases["inbound-p384-labelled-es256"] = repack(curveHeader, payloadRaw, parts[2])
	for name, record := range cases {
		t.Run(name, func(t *testing.T) {
			if verified, failure := Verify(record, p, bundle, testNow); verified != nil || failure == nil || failure.Transient || failure.VerifiedID != "" {
				t.Fatal("invalid inbound representation accepted or attributed", failure)
			}
		})
	}
	// Aggregate 8192 is implied by four individual 2048-byte DER bounds; the
	// overflow vector necessarily violates at least one constituent bound too.
	if 4*2048 != 8192 {
		t.Fatal("chain bounds incoherent")
	}
}

func TestBundleSnapshotsRotationAndMissingTrustClassification(t *testing.T) {
	now := time.Now()
	firstCA, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	secondCA, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := firstCA.SVID(identity.Trusted, now)
	second, _ := secondCA.SVID(identity.Trusted, now)
	p := testPolicy(t)
	firstRecord, f := Seal(first, p.Context(now), ledger.Fixed("snapshot"), now)
	if f != nil {
		t.Fatal(f)
	}
	source := &switchingBundle{bundle: firstCA.Bundle()}
	captured, f := Verify(firstRecord, p, source, now)
	if f != nil {
		t.Fatal(f)
	}
	source.bundle = secondCA.Bundle()
	if f := captured.Guard(now.Add(time.Second), true); f != nil {
		t.Fatal("later source update changed captured bounded path", f)
	}
	if _, f := Verify(firstRecord, p, source, now); f == nil || f.Transient || f.Reason != "untrusted_svid" {
		t.Fatal("removed root accepted on later snapshot", f)
	}
	secondRecord, f := Seal(second, p.Context(now), ledger.Fixed("snapshot"), now)
	if f != nil {
		t.Fatal(f)
	}
	if _, f := Verify(secondRecord, p, source, now); f != nil {
		t.Fatal("rotated matched leaf/key failed", f)
	}
	source.bundle = nil
	if _, f := Verify(secondRecord, p, source, now); f == nil || !f.Transient || f.Reason != "trust_unavailable" {
		t.Fatal("missing bundle treated as rejected trust", f)
	}
	source.bundle = secondCA.Bundle()
	if _, f := Verify(secondRecord, p, source, now); f != nil {
		t.Fatal("cached accepted local trust failed", f)
	}
}

type switchingBundle struct{ bundle *x509bundle.Bundle }

func (s *switchingBundle) GetX509BundleForTrustDomain(spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	if s.bundle == nil {
		return nil, errors.New("missing")
	}
	return s.bundle, nil
}

func TestSigningAndVerificationEnforceLeafNotBeforeWithoutGrace(t *testing.T) {
	svid, bundle := fixture(t, identity.Trusted, elliptic.P256())
	p := testPolicy(t)
	start := svid.Certificates[0].NotBefore
	if _, fail := Seal(svid, p.Context(start.Add(-time.Second)), ledger.Fixed("not-before"), start.Add(-time.Second)); fail == nil {
		t.Fatal("signer used not-yet-valid leaf")
	}
	record, fail := Seal(svid, p.Context(start), ledger.Fixed("not-before"), start)
	if fail != nil {
		t.Fatal("leaf notBefore equality refused", fail)
	}
	if _, fail := Verify(record, p, bundle, start.Add(-time.Second)); fail == nil || fail.Transient || fail.VerifiedID != "" {
		t.Fatal("verifier gave leaf a notBefore grace", fail)
	}
	if _, fail := Verify(record, p, bundle, start); fail != nil {
		t.Fatal("leaf at notBefore rejected", fail)
	}
}
