// Package testpki supplies in-memory certificate fixtures to protocol tests. It
// exports no key material and is not used by application startup or deployment.
package testpki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"math/big"
	"net/url"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

type CA struct {
	key    *ecdsa.PrivateKey
	cert   *x509.Certificate
	serial int64
}

func New(now time.Time) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	return &CA{key: key, cert: cert, serial: 1}, err
}
func (ca *CA) Bundle() *x509bundle.Bundle {
	return x509bundle.FromX509Authorities(spiffeid.RequireTrustDomainFromString("poc.internal"), []*x509.Certificate{ca.cert})
}
func (ca *CA) Cert(public any, id spiffeid.ID, now time.Time) (*x509.Certificate, error) {
	ca.serial++
	uri, _ := url.Parse(id.String())
	template := &x509.Certificate{SerialNumber: big.NewInt(ca.serial), URIs: []*url.URL{uri}, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(10 * time.Minute)}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, public, ca.key)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}
func (ca *CA) SVID(id spiffeid.ID, now time.Time) (*x509svid.SVID, error) {
	return ca.SVIDBetween(id, now.Add(-time.Minute), now.Add(10*time.Minute))
}
func (ca *CA) SVIDBetween(id spiffeid.ID, notBefore, notAfter time.Time) (*x509svid.SVID, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	ca.serial++
	uri, _ := url.Parse(id.String())
	template := &x509.Certificate{SerialNumber: big.NewInt(ca.serial), URIs: []*url.URL{uri}, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, NotBefore: notBefore, NotAfter: notAfter}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &x509svid.SVID{ID: id, Certificates: []*x509.Certificate{cert}, PrivateKey: key}, nil
}

type Source struct{ SVID *x509svid.SVID }

func (s Source) GetX509SVID() (*x509svid.SVID, error) {
	if s.SVID == nil {
		return nil, errors.New("no_svid")
	}
	return s.SVID, nil
}
