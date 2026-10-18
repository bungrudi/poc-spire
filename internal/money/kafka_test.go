package money

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

func TestKafkaOutcomeReadsOnlyObserverAndEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		h, ca, book := signedFixture(t, time.Now())
		var epoch uint64
		if enabled {
			book.EnableKafka(strings.Repeat("1", 32), strings.Repeat("2", 32))
			epoch = book.SetKafkaAssignment(true)
			book.RejectKafka(strings.Repeat("a", 64), "invalid_signature", "", epoch)
		}
		for _, peer := range []string{"", identity.Trusted.String(), identity.Observer.String(), "spiffe://poc.internal/test/wrong"} {
			for _, path := range []string{"/kafka/outcomes", "/kafka/outcomes/" + strings.Repeat("a", 64), "/kafka/outcomes/" + strings.Repeat("b", 64), "/kafka/outcomes/bad", "/kafka/outcomes/" + strings.Repeat("A", 64)} {
				w := logicalCall(t, h, ca, peer, "GET", path, "read", nil)
				expected := 403
				if peer == "" {
					expected = 401
				}
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
					if json.Unmarshal(responseBody(t, w, ca, h.clock()), &receipt) != nil || receipt.VerifiedProducer != "" || receipt.RequestID != "" || receipt.BusinessResult != nil {
						t.Fatal("invalid record claimed authenticated producer")
					}
				}
			}
		}
	}
}
func TestKafkaOutcomesRejectQueryBodyAndOtherMethods(t *testing.T) {
	h, ca, book := signedFixture(t, time.Now())
	book.EnableKafka("instance", "stream")
	for _, c := range []struct {
		method, path, body string
		status             int
	}{{"GET", "/kafka/outcomes?x=1", "", 400}, {"GET", "/kafka/outcomes", "{}", 400}, {"POST", "/kafka/outcomes", "", 403}, {"GET", "/kafka/outcomes/invalid", "", 403}, {"POST", "/transfer", "{}", 403}} {
		w := logicalCall(t, h, ca, identity.Observer.String(), c.method, c.path, "read", []byte(c.body))
		if w.Code != c.status {
			t.Fatal(c, w.Code)
		}
	}
}
