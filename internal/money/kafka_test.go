package money

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

func TestKafkaOutcomeReadsOnlyObserverAndEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		book := ledger.New(10)
		var epoch uint64
		if enabled {
			book.EnableKafka(strings.Repeat("1", 32), strings.Repeat("2", 32))
			epoch = book.SetKafkaAssignment(true)
			book.RejectKafka(strings.Repeat("a", 64), "invalid_signature", "", epoch)
		}
		h := &handler{ledger: book, audit: io.Discard}
		for _, peer := range []string{"", identity.Trusted.String(), identity.Observer.String(), "spiffe://poc.internal/test/wrong"} {
			for _, path := range []string{"/kafka/outcomes", "/kafka/outcomes/" + strings.Repeat("a", 64), "/kafka/outcomes/" + strings.Repeat("b", 64), "/kafka/outcomes/bad", "/kafka/outcomes/" + strings.Repeat("A", 64)} {
				r := httptest.NewRequest("GET", path, nil)
				if peer != "" {
					u, _ := url.Parse(peer)
					r.TLS = &tls.ConnectionState{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{{URIs: []*url.URL{u}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute)}}}
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				expected := 403
				if enabled && peer == identity.Observer.String() {
					switch path {
					case "/kafka/outcomes", "/kafka/outcomes/" + strings.Repeat("a", 64):
						expected = 200
					case "/kafka/outcomes/" + strings.Repeat("b", 64):
						expected = 404
					}
				}
				if w.Code != expected || book.Snapshot().Version != 0 {
					t.Fatal(enabled, peer, path, w.Code, expected)
				}
				if expected == 200 && strings.HasSuffix(path, strings.Repeat("a", 64)) {
					var receipt ledger.Receipt
					if json.Unmarshal(w.Body.Bytes(), &receipt) != nil || receipt.VerifiedProducer != "" || receipt.RequestID != "" || receipt.BusinessResult != nil {
						t.Fatal("invalid record claimed authenticated producer")
					}
				}
			}
		}
	}
}
func TestKafkaOutcomesRejectQueryBodyAndOtherMethods(t *testing.T) {
	book := ledger.New(10)
	book.EnableKafka("instance", "stream")
	h := &handler{ledger: book, audit: io.Discard}
	u, _ := url.Parse(identity.Observer.String())
	for _, c := range []struct {
		method, path, body string
		status             int
	}{{"GET", "/kafka/outcomes?x=1", "", 400}, {"GET", "/kafka/outcomes", "{}", 400}, {"POST", "/kafka/outcomes", "", 403}, {"GET", "/kafka/outcomes/invalid", "", 403}, {"POST", "/transfer", "{}", 403}} {
		r := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		r.TLS = &tls.ConnectionState{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{{URIs: []*url.URL{u}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute)}}}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.status {
			t.Fatal(c, w.Code)
		}
	}
}
