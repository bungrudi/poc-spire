package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBrowserCertificateValidityAndAtomicRenewal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser.pem")
	write := func(host string, expiry time.Time, serial int64) {
		t.Helper()
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host}, NotBefore: time.Now().Add(-time.Hour), NotAfter: expiry, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		private, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		data := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})...)
		if err := os.WriteFile(path+".new", data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".new", path); err != nil {
			t.Fatal(err)
		}
	}
	write("spire.adianto.dev", time.Now().Add(time.Hour), 1)
	first, err := browserCertificate(path, path, "https://spire.adianto.dev")
	if err != nil {
		t.Fatal(err)
	}
	write("spire.adianto.dev", time.Now().Add(time.Hour), 2)
	second, err := browserCertificate(path, path, "https://spire.adianto.dev")
	if err != nil {
		t.Fatal(err)
	}
	one, _ := x509.ParseCertificate(first.Certificate[0])
	two, _ := x509.ParseCertificate(second.Certificate[0])
	if one.SerialNumber.Cmp(two.SerialNumber) == 0 {
		t.Fatal("renewal did not reload the new generation")
	}
	write("other.example", time.Now().Add(time.Hour), 3)
	if _, err := browserCertificate(path, path, "https://spire.adianto.dev"); err == nil {
		t.Fatal("wrong hostname accepted")
	}
	write("spire.adianto.dev", time.Now().Add(-time.Minute), 4)
	if _, err := browserCertificate(path, path, "https://spire.adianto.dev"); err == nil {
		t.Fatal("expired browser certificate accepted")
	}
}
