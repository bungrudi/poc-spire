package kafkacmd

import (
	"crypto/x509"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

type Verified struct {
	Message  Message
	Producer string
	path     []*x509.Certificate
	started  time.Time
}

// Verify takes a local immutable bundle snapshot before cryptographic work.
// Source acquisition failure differs from rejection against available trust.
func Verify(record Record, policy Policy, source x509bundle.Source, now time.Time) (*Verified, *Failure) {
	if !policy.Valid() {
		return nil, unavailable("invalid_consumer_policy")
	}
	started := now
	parsed, fail := Parse(record)
	if fail != nil {
		return nil, fail
	}
	if source == nil {
		return nil, unavailable("trust_unavailable")
	}
	bundle, err := source.GetX509BundleForTrustDomain(identity.Trusted.TrustDomain())
	if err != nil || bundle == nil || len(bundle.X509Authorities()) == 0 {
		return nil, unavailable("trust_unavailable")
	}
	authorities := []*x509.Certificate{}
	for _, cert := range bundle.X509Authorities() {
		clone, err := x509.ParseCertificate(cert.Raw)
		if err != nil {
			return nil, unavailable("trust_unavailable")
		}
		authorities = append(authorities, clone)
	}
	snapshot := x509bundle.FromX509Authorities(identity.Trusted.TrustDomain(), authorities)
	if !leafRules(parsed.Certificates[0]) {
		return nil, failure("invalid_svid_key")
	}
	id, paths, err := x509svid.Verify(parsed.Certificates, snapshot, x509svid.WithTime(now))
	if err != nil || len(paths) == 0 {
		return nil, failure("untrusted_svid")
	}
	// Go x509 verification treats NotAfter equality inclusively; this profile
	// requires the strict current interval before asserting a signer identity.
	var path []*x509.Certificate
	for _, candidate := range paths {
		current := true
		for _, cert := range candidate {
			if now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
				current = false
				break
			}
		}
		if current {
			path = candidate
			break
		}
	}
	if path == nil {
		return nil, failure("credential_not_current")
	}
	jws, err := jose.ParseSignedCompact(parsed.Compact, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		return nil, failure("malformed_jws")
	}
	if _, err := jws.Verify(parsed.Certificates[0].PublicKey); err != nil {
		return nil, failure("invalid_signature")
	}
	verified := &Verified{parsed.Message, id.String(), path, started}
	reject := func(reason string) (*Verified, *Failure) {
		return nil, &Failure{Reason: reason, VerifiedID: id.String()}
	}
	if id != identity.Trusted {
		return reject("unauthorized_signer")
	}
	c := parsed.Message.Context
	if c.Version != 1 || c.Environment != Environment || c.StreamID != policy.StreamID || c.Topic != policy.Topic || c.Audience != identity.Money.String() || c.Type != CommandType || record.Topic != c.Topic || !RequestID.MatchString(parsed.Message.Transfer.RequestID) || string(record.Key) != parsed.Message.Transfer.RequestID {
		return reject("context_mismatch")
	}
	if fail := verified.Guard(now, true); fail != nil {
		return nil, fail
	}
	return verified, nil
}

// Guard performs only local interval/ownership checks. It is safe to execute
// under the shared acceptance lock; no certificate verification or I/O occurs.
func (v *Verified) Guard(now time.Time, assigned bool) *Failure {
	transient := func(reason string) *Failure { return &Failure{Reason: reason, Transient: true, VerifiedID: v.Producer} }
	terminal := func(reason string) *Failure { return &Failure{Reason: reason, VerifiedID: v.Producer} }
	if !assigned {
		return transient("assignment_lost")
	}
	if now.Sub(v.started) > 2*time.Second || now.Before(v.started) {
		return transient("verification_budget_exhausted")
	}
	for _, cert := range v.path {
		if now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
			return terminal("credential_not_current")
		}
	}
	m := v.Message
	c := m.Context
	leaf := v.path[0]
	if c.Deadline <= c.CreatedAt || c.Deadline-c.CreatedAt > 120 || c.CreatedAt > m.IssuedAt || m.ExpiresAt <= m.IssuedAt || m.ExpiresAt-m.IssuedAt > 30 || m.ExpiresAt > c.Deadline || m.IssuedAt < leaf.NotBefore.Unix() || !time.Unix(m.IssuedAt, 0).Before(leaf.NotAfter) || m.IssuedAt > now.Unix()+5 {
		return terminal("invalid_message_time")
	}
	if now.Unix() >= m.ExpiresAt || now.Unix() >= c.Deadline {
		return terminal("message_expired")
	}
	return nil
}
func (v *Verified) LedgerGuard(clock func() time.Time, assigned func() bool) func() *ledger.GuardFailure {
	return func() *ledger.GuardFailure {
		fail := v.Guard(clock(), assigned())
		if fail == nil {
			return nil
		}
		return &ledger.GuardFailure{Reason: fail.Reason, Transient: fail.Transient}
	}
}
