package kafkaio

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestBrokerObservationRequiresCompletedDefaultVerifiedHandshake(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"kafka"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 2; i++ {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			secure := tls.Server(raw, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}})
			secure.Handshake()
			secure.Close()
		}
	}()
	var audit bytes.Buffer
	observer := &tlsObservation{audit: &audit}
	connection, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", listener.Addr().String(), &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: "kafka"})
	if err != nil {
		t.Fatal(err)
	}
	observer.OnBrokerConnect(kgo.BrokerMetadata{}, 0, connection, nil)
	var event map[string]any
	if err = json.Unmarshal(audit.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event["verified_peer_name"] != "kafka" || event["tls_version"] != "TLS 1.3" || event["tls_session_resumed"] != false || event["local"] != connection.LocalAddr().String() || event["remote"] != connection.RemoteAddr().String() {
		t.Fatal(event)
	}
	if _, err = time.Parse(time.RFC3339Nano, event["at_utc"].(string)); err != nil {
		t.Fatal(err)
	}
	if len(event) != 7 {
		t.Fatal("unexpected connection observation fields", event)
	}
	connection.Close()
	audit.Reset()
	failed, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", listener.Addr().String(), &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: "wrong.invalid"})
	if err == nil {
		failed.Close()
		t.Fatal("hostname verification changed")
	}
	observer.OnBrokerConnect(kgo.BrokerMetadata{}, 0, failed, err)
	if audit.Len() != 0 {
		t.Fatal("failed verification was reported as authenticated connection")
	}
	raw, other := net.Pipe()
	defer raw.Close()
	defer other.Close()
	observer.OnBrokerConnect(kgo.BrokerMetadata{}, 0, raw, nil)
	observer.OnBrokerConnect(kgo.BrokerMetadata{}, 0, nil, errors.New("TOKEN_SECRET"))
	if audit.Len() != 0 {
		t.Fatal("raw TCP/error contents produced TLS success evidence")
	}
	<-done
}
