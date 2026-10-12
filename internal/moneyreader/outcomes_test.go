package moneyreader

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
	"poc-spire/internal/testpki"
)

func readerServer(t *testing.T, serverID, clientID spiffeid.ID, reply http.HandlerFunc) (Observer, *httptest.Server) {
	t.Helper()
	now := time.Now()
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	serverSVID, err := ca.SVID(serverID, now)
	if err != nil {
		t.Fatal(err)
	}
	var clientSVID *x509svid.SVID
	if !clientID.IsZero() {
		clientSVID, err = ca.SVID(clientID, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewUnstartedServer(reply)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = identity.ServerTLS(testpki.Source{SVID: serverSVID}, ca.Bundle())
	server.TLS.Certificates = []tls.Certificate{{Certificate: [][]byte{serverSVID.Certificates[0].Raw}, PrivateKey: serverSVID.PrivateKey}}
	server.TLS.NextProtos = []string{"http/1.1"}
	server.Config.TLSNextProto = map[string]func(*http.Server, *tls.Conn, http.Handler){}
	server.StartTLS()
	t.Cleanup(server.Close)
	return Observer{Target: server.URL, SVID: testpki.Source{SVID: clientSVID}, Bundle: ca.Bundle()}, server
}
func TestObserverTLSAuthenticatesMoneyAndNoCertificateNeverReachesHTTP(t *testing.T) {
	for _, mode := range []string{"valid", "wrong-server", "no-cert"} {
		t.Run(mode, func(t *testing.T) {
			serverID, clientID := identity.Money, identity.Observer
			if mode == "wrong-server" {
				serverID = identity.Trusted
			}
			if mode == "no-cert" {
				clientID = spiffeid.ID{}
			}
			calls := 0
			observer, _ := readerServer(t, serverID, clientID, func(w http.ResponseWriter, r *http.Request) {
				calls++
				id, err := x509svid.IDFromCert(r.TLS.PeerCertificates[0])
				if err != nil || id != identity.Observer {
					w.WriteHeader(403)
					return
				}
				json.NewEncoder(w).Encode(ledger.KafkaMetadata{InstanceID: strings.Repeat("a", 32), StreamID: strings.Repeat("b", 32), ConsumerReady: true})
			})
			m, err := observer.Metadata(context.Background())
			if mode == "valid" {
				if err != nil || !m.ConsumerReady || calls != 1 {
					t.Fatal(m, err, calls)
				}
			} else if err == nil || calls != 0 {
				t.Fatal("unverified Money HTTP evidence", mode, calls)
			}
		})
	}
}
func TestReceiptReadIsBoundedCorrelatedAndSeparatesHTTPStatus(t *testing.T) {
	digest := strings.Repeat("a", 64)
	for _, mode := range []string{"valid", "missing", "wrong-digest", "duplicate", "oversize", "wrong-reader"} {
		t.Run(mode, func(t *testing.T) {
			clientID := identity.Observer
			if mode == "wrong-reader" {
				clientID = identity.Trusted
			}
			observer, _ := readerServer(t, identity.Money, clientID, func(w http.ResponseWriter, r *http.Request) {
				id, _ := x509svid.IDFromCert(r.TLS.PeerCertificates[0])
				if id != identity.Observer {
					w.WriteHeader(403)
					return
				}
				switch mode {
				case "oversize":
					io.WriteString(w, strings.Repeat("x", 8193))
				case "duplicate":
					io.WriteString(w, `{"instance_id":"`+strings.Repeat("b", 32)+`","instance_id":"`+strings.Repeat("c", 32)+`"}`)
				case "missing":
					w.WriteHeader(404)
					json.NewEncoder(w).Encode(ledger.KafkaMetadata{InstanceID: strings.Repeat("b", 32), StreamID: strings.Repeat("c", 32), ConsumerReady: true})
				default:
					d := digest
					if mode == "wrong-digest" {
						d = strings.Repeat("d", 64)
					}
					now := time.Now().Unix()
					json.NewEncoder(w).Encode(ledger.Receipt{InstanceID: strings.Repeat("b", 32), Digest: d, ObservedAt: now, RetainUntil: now + 180, Disposition: "rejected", Reason: "invalid_signature"})
				}
			})
			receipt, m, code, err := observer.Receipt(context.Background(), digest)
			switch mode {
			case "valid":
				if err != nil || code != 200 || receipt.BusinessCode != 0 || receipt.VerifiedProducer != "" || m.InstanceID == "" {
					t.Fatal(receipt, code, err)
				}
			case "missing":
				if err != nil || code != 404 || m.InstanceID == "" || receipt.Digest != "" {
					t.Fatal("404 invented a terminal result")
				}
			default:
				if err == nil {
					t.Fatal("uncorrelated evidence accepted", mode)
				}
			}
		})
	}
}
