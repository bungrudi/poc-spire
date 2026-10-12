package kafkacmd

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/cryptosigner"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

type Parsed struct {
	Message      Message
	Certificates []*x509.Certificate
	Compact      string
}

func segment(s string, limit int) ([]byte, error) {
	if s == "" || len(s) > (limit*4+2)/3 {
		return nil, failure("malformed_jws")
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil || len(b) > limit || base64.RawURLEncoding.EncodeToString(b) != s {
		return nil, failure("malformed_jws")
	}
	return b, nil
}
func Parse(r Record) (*Parsed, *Failure) {
	if len(r.Value) > MaxValue || len(r.Key) > 128 {
		return nil, failure("record_bounds")
	}
	if len(r.Headers) != 0 {
		return nil, failure("unsupported_kafka_headers")
	}
	parts := strings.Split(string(r.Value), ".")
	if len(parts) != 3 {
		return nil, failure("malformed_jws")
	}
	rawHeader, err := segment(parts[0], MaxHeader)
	if err != nil {
		return nil, failure("malformed_jws")
	}
	rawPayload, err := segment(parts[1], MaxPayload)
	if err != nil {
		return nil, failure("malformed_jws")
	}
	sig, err := segment(parts[2], 64)
	if err != nil || len(sig) != 64 {
		return nil, failure("malformed_signature")
	}
	header, err := StrictJSON(rawHeader)
	if err != nil || !exactKeys(header, "alg", "typ", "x5c") {
		return nil, failure("malformed_header")
	}
	if header["alg"] != "ES256" || header["typ"] != Type {
		return nil, failure("unsupported_header")
	}
	chain, ok := header["x5c"].([]any)
	if !ok || len(chain) < 1 || len(chain) > 4 {
		return nil, failure("certificate_bounds")
	}
	certs := make([]*x509.Certificate, 0, len(chain))
	total := 0
	for _, entry := range chain {
		s, ok := entry.(string)
		if !ok || len(s) > 2732 {
			return nil, failure("certificate_bounds")
		}
		der, err := base64.StdEncoding.Strict().DecodeString(s)
		total += len(der)
		if err != nil || len(der) > 2048 || total > 8192 || base64.StdEncoding.EncodeToString(der) != s {
			return nil, failure("malformed_certificate")
		}
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, failure("malformed_certificate")
		}
		certs = append(certs, c)
	}
	for i := 1; i < len(certs); i++ {
		if certs[i-1].CheckSignatureFrom(certs[i]) != nil {
			return nil, failure("unordered_certificate_chain")
		}
	}
	msg, err := parsePayload(rawPayload)
	if err != nil {
		return nil, failure("malformed_payload")
	}
	return &Parsed{msg, certs, string(r.Value)}, nil
}
func parsePayload(raw []byte) (Message, error) {
	obj, err := StrictJSON(raw)
	if err != nil || !exactKeys(obj, "context", "transfer", "issued_at", "expires_at") {
		return Message{}, failure("malformed_payload")
	}
	c, ok := obj["context"].(map[string]any)
	if !ok || !exactKeys(c, "version", "environment", "stream_id", "topic", "audience", "type", "created_at", "deadline") {
		return Message{}, failure("malformed_context")
	}
	t, ok := obj["transfer"].(map[string]any)
	if !ok || !exactKeys(t, "request_id", "from", "to", "amount_minor", "currency") {
		return Message{}, failure("malformed_transfer")
	}
	for _, name := range []string{"environment", "stream_id", "topic", "audience", "type"} {
		if _, ok := stringField(c, name); !ok {
			return Message{}, failure("context_type")
		}
	}
	for _, name := range []string{"request_id", "from", "to", "currency"} {
		if _, ok := stringField(t, name); !ok {
			return Message{}, failure("transfer_type")
		}
	}
	for _, name := range []string{"version", "created_at", "deadline"} {
		if _, ok := intField(c, name); !ok {
			return Message{}, failure("context_type")
		}
	}
	if _, ok := intField(t, "amount_minor"); !ok {
		return Message{}, failure("transfer_type")
	}
	for _, name := range []string{"issued_at", "expires_at"} {
		if _, ok := intField(obj, name); !ok {
			return Message{}, failure("envelope_type")
		}
	}
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		return Message{}, err
	}
	if !timestamp(msg.IssuedAt) || !timestamp(msg.ExpiresAt) || !timestamp(msg.Context.CreatedAt) || !timestamp(msg.Context.Deadline) {
		return Message{}, failure("timestamp_range")
	}
	return msg, nil
}
func p256(cert *x509.Certificate) (*ecdsa.PublicKey, bool) {
	key, ok := cert.PublicKey.(*ecdsa.PublicKey)
	return key, ok && key.Curve == elliptic.P256() && key.Curve.IsOnCurve(key.X, key.Y)
}
func leafRules(cert *x509.Certificate) bool {
	_, ok := p256(cert)
	return ok && !cert.IsCA && cert.KeyUsage&x509.KeyUsageDigitalSignature != 0 && cert.KeyUsage&(x509.KeyUsageCertSign|x509.KeyUsageCRLSign) == 0
}

// Seal uses the one matched SDK object; it never exports or independently
// refreshes its private key. Callers may retry acquisition for at most ten seconds.
func Seal(svid *x509svid.SVID, context Context, transfer ledger.Request, now time.Time) (Record, *Failure) {
	if svid == nil || svid.ID != identity.Trusted || svid.PrivateKey == nil || len(svid.Certificates) == 0 {
		return Record{}, unavailable("signing_svid_unavailable")
	}
	leaf := svid.Certificates[0]
	id, err := x509svid.IDFromCert(leaf)
	if err != nil || id != svid.ID || !leafRules(leaf) || now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return Record{}, unavailable("signing_svid_incompatible")
	}
	a, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
	if err != nil {
		return Record{}, unavailable("signing_key_mismatch")
	}
	b, err := x509.MarshalPKIXPublicKey(svid.PrivateKey.Public())
	if err != nil || !bytes.Equal(a, b) {
		return Record{}, unavailable("signing_key_mismatch")
	}
	expires := min(now.Unix()+30, context.Deadline, leaf.NotAfter.Unix()-5)
	if expires-now.Unix() <= 5 || now.Unix() < context.CreatedAt {
		return Record{}, unavailable("signing_lifetime_unavailable")
	}
	chain := []string{}
	total := 0
	for _, cert := range svid.Certificates {
		total += len(cert.Raw)
		if len(cert.Raw) > 2048 || total > 8192 {
			return Record{}, unavailable("signing_chain_bounds")
		}
		chain = append(chain, base64.StdEncoding.EncodeToString(cert.Raw))
	}
	if len(chain) > 4 {
		return Record{}, unavailable("signing_chain_bounds")
	}
	opts := (&jose.SignerOptions{}).WithType(jose.ContentType(Type)).WithHeader(jose.HeaderKey("x5c"), chain)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: cryptosigner.Opaque(svid.PrivateKey)}, opts)
	if err != nil {
		return Record{}, unavailable("signing_failed")
	}
	msg := Message{context, transfer, now.Unix(), expires}
	raw, err := json.Marshal(msg)
	if err != nil || len(raw) > MaxPayload {
		return Record{}, unavailable("signing_payload_bounds")
	}
	jws, err := signer.Sign(raw)
	if err != nil {
		return Record{}, unavailable("signing_failed")
	}
	compact, err := jws.CompactSerialize()
	if err != nil {
		return Record{}, unavailable("signing_failed")
	}
	record := Record{Topic: context.Topic, Key: []byte(transfer.RequestID), Value: []byte(compact)}
	if _, fail := Parse(record); fail != nil {
		return Record{}, unavailable("sealed_profile_invalid")
	}
	return record, nil
}

// FixedRecord exposes only the reviewed menu. Tampering retains the original
// signature and changes one typed business value deterministically.
func FixedRecord(svid *x509svid.SVID, context Context, id, scenario string, now time.Time) (Record, *Failure) {
	if !RequestID.MatchString(id) {
		return Record{}, failure("invalid_request_id")
	}
	if scenario == "kafka-unidentified" {
		raw, _ := json.Marshal(Message{context, ledger.Fixed(id), now.Unix(), min(now.Unix()+30, context.Deadline)})
		return Record{Topic: context.Topic, Key: []byte(id), Value: raw}, nil
	}
	if scenario != "kafka-trusted" && scenario != "kafka-tampered" {
		return Record{}, failure("invalid_scenario")
	}
	record, fail := Seal(svid, context, ledger.Fixed(id), now)
	if fail != nil {
		return Record{}, fail
	}
	if scenario == "kafka-tampered" {
		parts := strings.Split(string(record.Value), ".")
		raw, _ := segment(parts[1], MaxPayload)
		var msg Message
		json.Unmarshal(raw, &msg)
		msg.Transfer.Amount = 200
		raw, _ = json.Marshal(msg)
		parts[1] = base64.RawURLEncoding.EncodeToString(raw)
		record.Value = []byte(strings.Join(parts, "."))
	}
	return record, nil
}
