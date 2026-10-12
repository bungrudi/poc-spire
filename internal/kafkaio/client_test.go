package kafkaio

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

	"poc-spire/internal/ledger"
)

func brokerFixture(t *testing.T) Config {
	t.Helper()
	dir := t.TempDir()
	now := time.Now()
	rootKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rootT := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fixture-ca"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootT, rootT, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	root, _ := x509.ParseCertificate(rootDER)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leafT := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "kafka-money"}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafT, root, &key.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"ca.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}), "client.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), "client.key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})} {
		if os.WriteFile(filepath.Join(dir, name), data, 0600) != nil {
			t.Fatal("temporary broker credential unavailable")
		}
	}
	return Config{StreamID: "0123456789abcdef0123456789abcdef", Brokers: []string{"kafka:9093"}, CA: filepath.Join(dir, "ca.pem"), Certificate: filepath.Join(dir, "client.pem"), Key: filepath.Join(dir, "client.key")}
}
func TestRealProducerAndConsumerConstructWithCompatibleBoundedOptions(t *testing.T) {
	cfg := brokerFixture(t)
	producer, err := Producer(cfg)
	if err != nil {
		t.Fatal("real producer option validation", err)
	}
	producer.Close()
	book := ledger.New(10)
	book.EnableKafka("instance", cfg.StreamID)
	consumer, err := NewConsumer(cfg, book, nil, nil)
	if err != nil {
		t.Fatal("real consumer option validation", err)
	}
	consumer.Close()
}
