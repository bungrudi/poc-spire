package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

type cached struct {
	svid   *x509svid.SVID
	bundle *x509bundle.Bundle
}

func (c cached) GetX509SVID() (*x509svid.SVID, error) { return c.svid, nil }
func (c cached) GetX509BundleForTrustDomain(td spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return c.bundle.GetX509BundleForTrustDomain(td)
}

func certificate(t *testing.T, template, parent *x509.Certificate, key, signer *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	raw, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, signer)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func material(t *testing.T, rootExpiry, leafExpiry time.Time) cached {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "own root"}, NotBefore: now.Add(-time.Hour), NotAfter: rootExpiry, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	root = certificate(t, root, root, key, key)
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: now.Add(-time.Minute), NotAfter: leafExpiry, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, URIs: []*url.URL{Trusted.URL()}}
	leaf = certificate(t, leaf, root, leafKey, key)
	return cached{&x509svid.SVID{ID: Trusted, Certificates: []*x509.Certificate{leaf}, PrivateKey: leafKey}, x509bundle.FromX509Authorities(Trusted.TrustDomain(), []*x509.Certificate{root})}
}

func TestCurrentUsesAuthenticCurrentCachedPath(t *testing.T) {
	now := time.Now()
	own := material(t, now.Add(time.Hour), now.Add(time.Minute))
	// This source serves only cached material. No issuer connection exists or is required.
	if !Current(own) {
		t.Fatal("usable cached identity denied during issuer outage")
	}
	unrelated := material(t, now.Add(time.Hour), now.Add(time.Minute))
	empty := own
	empty.bundle = x509bundle.New(Trusted.TrustDomain())
	wrong := own
	wrong.bundle = unrelated.bundle
	wrongID := own
	svid := *own.svid
	svid.ID = Observer
	wrongID.svid = &svid
	for name, source := range map[string]cached{
		"empty trust": empty, "unrelated trust": wrong, "metadata ID mismatch": wrongID,
		"expired anchor": material(t, now.Add(-time.Second), now.Add(time.Minute)),
		"expired leaf":   material(t, now.Add(time.Hour), now.Add(-time.Second)),
	} {
		t.Run(name, func(t *testing.T) {
			if Current(source) {
				t.Fatal("unusable identity/trust marked ready")
			}
		})
	}
	expired := material(t, now.Add(-time.Second), now.Add(time.Minute))
	roots := append(own.bundle.X509Authorities(), expired.bundle.X509Authorities()...)
	own.bundle = x509bundle.FromX509Authorities(Trusted.TrustDomain(), roots)
	if !Current(own) {
		t.Fatal("usable current path denied by unrelated retained expired root")
	}
}
