// Package httpjws authenticates Kubernetes workload HTTP with a compact
// X.509-SVID JWS. It does not terminate TLS.
package httpjws

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/cryptosigner"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacmd"
)

const (
	Type       = "application/vnd.poc-spire.http-payload.v1+jws"
	MaxValue   = 32768
	MaxHeader  = 16384
	MaxPayload = 4096
)

type Envelope struct {
	Method    string          `json:"method"`
	Path      string          `json:"path"`
	RequestID string          `json:"request_id"`
	IssuedAt  int64           `json:"issued_at"`
	ExpiresAt int64           `json:"expires_at"`
	Body      json.RawMessage `json:"body"`
}

type Verified struct {
	Envelope
	Signer spiffeid.ID
	leaf   *x509.Certificate
	path   []*x509.Certificate
}

type Failure struct {
	Reason string
	Status int
}

func (v *Verified) Current(now time.Time) bool {
	return v != nil && fresh(v.leaf, v.path, v.IssuedAt, v.ExpiresAt, now)
}

func Sign(svid *x509svid.SVID, method, path, requestID string, body json.RawMessage, now time.Time) (string, error) {
	if svid == nil || svid.PrivateKey == nil || len(svid.Certificates) == 0 || !kafkacmd.RequestID.MatchString(requestID) {
		return "", errUnavailable
	}
	leaf := svid.Certificates[0]
	id, err := x509svid.IDFromCert(leaf)
	if err != nil || id != svid.ID || !leafRules(leaf) || now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return "", errUnavailable
	}
	signer, ok := svid.PrivateKey.(crypto.Signer)
	if !ok || !keyMatches(leaf, signer) {
		return "", errUnavailable
	}
	expires := min(now.Unix()+30, leaf.NotAfter.Unix()-5)
	if expires-now.Unix() <= 5 {
		return "", errUnavailable
	}
	return seal(svid, method, path, requestID, body, now.Unix(), expires)
}

func Verify(compact string, source x509bundle.Source, now time.Time) (*Verified, *Failure) {
	parsed, fail := parse(compact)
	if fail != nil {
		return nil, fail
	}
	if source == nil {
		return nil, &Failure{Reason: "trust_unavailable", Status: 503}
	}
	bundle, err := source.GetX509BundleForTrustDomain(identity.Money.TrustDomain())
	if err != nil || bundle == nil || len(bundle.X509Authorities()) == 0 {
		return nil, &Failure{Reason: "trust_unavailable", Status: 503}
	}
	authorities := make([]*x509.Certificate, 0, len(bundle.X509Authorities()))
	for _, cert := range bundle.X509Authorities() {
		clone, err := x509.ParseCertificate(cert.Raw)
		if err != nil {
			return nil, &Failure{Reason: "trust_unavailable", Status: 503}
		}
		authorities = append(authorities, clone)
	}
	snapshot := x509bundle.FromX509Authorities(identity.Money.TrustDomain(), authorities)
	trustAt := now
	leaf := parsed.certs[0]
	if now.Before(leaf.NotBefore) {
		trustAt = leaf.NotBefore
	}
	if !now.Before(leaf.NotAfter) {
		trustAt = leaf.NotAfter.Add(-time.Second)
	}
	id, paths, err := x509svid.Verify(parsed.certs, snapshot, x509svid.WithTime(trustAt))
	if err != nil || len(paths) == 0 {
		return nil, &Failure{Reason: "untrusted_svid", Status: 401}
	}
	var path []*x509.Certificate
	for _, candidate := range paths {
		if chainCurrent(candidate, trustAt) {
			path = candidate
			break
		}
	}
	if path == nil {
		return nil, &Failure{Reason: "untrusted_svid", Status: 401}
	}
	verified := &Verified{Envelope: parsed.envelope, Signer: id, leaf: leaf, path: path}
	if !fresh(leaf, path, parsed.envelope.IssuedAt, parsed.envelope.ExpiresAt, now) {
		return nil, &Failure{Reason: "credential_not_current", Status: 403}
	}
	return verified, nil
}

type parsedJWS struct {
	envelope Envelope
	certs    []*x509.Certificate
}

func parse(compact string) (*parsedJWS, *Failure) {
	if compact == "" {
		return nil, &Failure{Reason: "invalid_signature", Status: 401}
	}
	if len(compact) > MaxValue {
		return nil, &Failure{Reason: "malformed_jws", Status: 400}
	}
	parts := strings.Split(compact, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, &Failure{Reason: "invalid_signature", Status: 401}
	}
	rawHeader, err := segment(parts[0], MaxHeader)
	if err != nil {
		return nil, &Failure{Reason: "malformed_jws", Status: 400}
	}
	rawPayload, err := segment(parts[1], MaxPayload)
	if err != nil {
		return nil, &Failure{Reason: "malformed_jws", Status: 400}
	}
	sig, err := segment(parts[2], 64)
	if err != nil || len(sig) != 64 {
		return nil, &Failure{Reason: "invalid_signature", Status: 401}
	}
	header, err := kafkacmd.StrictJSON(rawHeader)
	if err != nil {
		return nil, &Failure{Reason: "malformed_jws", Status: 400}
	}
	alg, _ := header["alg"].(string)
	if alg == "" || alg != "ES256" {
		return nil, &Failure{Reason: "invalid_signature", Status: 401}
	}
	certs, fail := parseChain(header["x5c"])
	if fail != nil {
		return nil, fail
	}
	if !leafRules(certs[0]) {
		return nil, &Failure{Reason: "invalid_signature", Status: 401}
	}
	if _, err = verifySignature(compact, certs[0]); err != nil {
		return nil, &Failure{Reason: "invalid_signature", Status: 401}
	}
	if len(header) != 3 || header["typ"] != Type {
		return nil, &Failure{Reason: "malformed_jws", Status: 400}
	}
	env, fail := parseEnvelope(rawPayload)
	if fail != nil {
		return nil, fail
	}
	return &parsedJWS{envelope: env, certs: certs}, nil
}

func parseEnvelope(raw []byte) (Envelope, *Failure) {
	malformed := &Failure{Reason: "malformed_jws", Status: 400}
	if duplicateKeys(raw) {
		return Envelope{}, malformed
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var env Envelope
	if err := dec.Decode(&env); err != nil {
		return Envelope{}, malformed
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Envelope{}, malformed
	}
	if env.Method == "" || env.Path == "" || strings.ContainsAny(env.Method, " \r\n") || strings.ContainsAny(env.Path, " \r\n?") || !kafkacmd.RequestID.MatchString(env.RequestID) {
		return Envelope{}, malformed
	}
	if env.IssuedAt < 0 || env.ExpiresAt < 0 || env.IssuedAt > 4102444800 || env.ExpiresAt > 4102444800 {
		return Envelope{}, malformed
	}
	if len(env.Body) == 0 || !json.Valid(env.Body) {
		return Envelope{}, malformed
	}
	if string(env.Body) != "null" {
		if _, err := kafkacmd.StrictJSON(env.Body); err != nil {
			return Envelope{}, malformed
		}
	}
	return env, nil
}

func parseChain(value any) ([]*x509.Certificate, *Failure) {
	chain, ok := value.([]any)
	if !ok || len(chain) < 1 || len(chain) > 4 {
		return nil, &Failure{Reason: "malformed_jws", Status: 400}
	}
	certs := make([]*x509.Certificate, 0, len(chain))
	total := 0
	for _, entry := range chain {
		s, ok := entry.(string)
		if !ok || len(s) > 2732 {
			return nil, &Failure{Reason: "malformed_jws", Status: 400}
		}
		der, err := base64.StdEncoding.Strict().DecodeString(s)
		total += len(der)
		if err != nil || len(der) > 2048 || total > 8192 || base64.StdEncoding.EncodeToString(der) != s {
			return nil, &Failure{Reason: "malformed_jws", Status: 400}
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, &Failure{Reason: "malformed_jws", Status: 400}
		}
		certs = append(certs, cert)
	}
	for i := 1; i < len(certs); i++ {
		if certs[i-1].CheckSignatureFrom(certs[i]) != nil {
			return nil, &Failure{Reason: "malformed_jws", Status: 400}
		}
	}
	return certs, nil
}

func seal(svid *x509svid.SVID, method, path, requestID string, body json.RawMessage, issued, expires int64) (string, error) {
	if len(body) == 0 {
		body = json.RawMessage("null")
	}
	chain := []string{}
	total := 0
	for _, cert := range svid.Certificates {
		total += len(cert.Raw)
		if len(cert.Raw) > 2048 || total > 8192 || len(chain) == 4 {
			return "", errUnavailable
		}
		chain = append(chain, base64.StdEncoding.EncodeToString(cert.Raw))
	}
	opts := (&jose.SignerOptions{}).WithType(jose.ContentType(Type)).WithHeader(jose.HeaderKey("x5c"), chain)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: cryptosigner.Opaque(svid.PrivateKey)}, opts)
	if err != nil {
		return "", errUnavailable
	}
	raw, err := json.Marshal(Envelope{method, path, requestID, issued, expires, body})
	if err != nil || len(raw) > MaxPayload {
		return "", errUnavailable
	}
	signed, err := signer.Sign(raw)
	if err != nil {
		return "", errUnavailable
	}
	compact, err := signed.CompactSerialize()
	if err != nil || len(compact) > MaxValue {
		return "", errUnavailable
	}
	return compact, nil
}

func verifySignature(compact string, leaf *x509.Certificate) ([]byte, error) {
	jws, err := jose.ParseSignedCompact(compact, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		return nil, err
	}
	return jws.Verify(leaf.PublicKey)
}

func fresh(leaf *x509.Certificate, path []*x509.Certificate, issued, expires int64, now time.Time) bool {
	if leaf == nil || expires <= issued || expires-issued > 30 || issued > now.Unix()+5 || now.Unix() >= expires {
		return false
	}
	if issued < leaf.NotBefore.Unix() || !time.Unix(issued, 0).Before(leaf.NotAfter) {
		return false
	}
	certs := path
	if len(certs) == 0 {
		certs = []*x509.Certificate{leaf}
	}
	for _, cert := range certs {
		if cert.NotBefore.After(now.Add(5*time.Second)) || !now.Before(cert.NotAfter) {
			return false
		}
	}
	return true
}

func chainCurrent(chain []*x509.Certificate, now time.Time) bool {
	for _, cert := range chain {
		if now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
			return false
		}
	}
	return len(chain) > 0
}

func leafRules(cert *x509.Certificate) bool {
	key, ok := cert.PublicKey.(*ecdsa.PublicKey)
	return ok && key.Curve == elliptic.P256() && key.Curve.IsOnCurve(key.X, key.Y) && !cert.IsCA && cert.KeyUsage&x509.KeyUsageDigitalSignature != 0 && cert.KeyUsage&(x509.KeyUsageCertSign|x509.KeyUsageCRLSign) == 0
}

func keyMatches(leaf *x509.Certificate, private crypto.Signer) bool {
	if private == nil {
		return false
	}
	a, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
	if err != nil {
		return false
	}
	b, err := x509.MarshalPKIXPublicKey(private.Public())
	return err == nil && bytes.Equal(a, b)
}

func duplicateKeys(raw []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return true
	}
	seen := map[string]bool{}
	for dec.More() {
		token, err = dec.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return true
		}
		seen[name] = true
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			return true
		}
	}
	end, err := dec.Token()
	return err != nil || end != json.Delim('}')
}

func segment(s string, limit int) ([]byte, error) {
	if s == "" || len(s) > (limit*4+2)/3 {
		return nil, errMalformed
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil || len(b) > limit || base64.RawURLEncoding.EncodeToString(b) != s {
		return nil, errMalformed
	}
	return b, nil
}

type unavailableError struct{}

func (unavailableError) Error() string { return "signing_svid_unavailable" }

var errUnavailable = unavailableError{}
var errMalformed = unavailableError{}
