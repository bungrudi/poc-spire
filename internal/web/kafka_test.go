package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"poc-spire/internal/control"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/ledger"
)

const streamTest = "0123456789abcdef0123456789abcdef"
const instanceTest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func kafkaBody(key, scenario string) []byte {
	raw, _ := json.Marshal(map[string]string{"scenario": scenario, "browser_key": key, "stream_id": streamTest})
	return raw
}
func newKafkaApp(t *testing.T, ctx context.Context, config Config, deps KafkaDependencies) *App {
	t.Helper()
	config.Origin = "http://127.0.0.1:8080"
	config.Local = true
	config.KafkaStream = streamTest
	a, err := New(ctx, config, Dependencies{Execute: func(_ context.Context, _, id string) control.Result {
		return control.Result{RequestID: id, Outcome: "transfer_succeeded"}
	}, Balances: func(context.Context) (any, error) { return map[string]int{"version": 0}, nil }, Kafka: &deps})
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func fixtureMetadata(id, scenario string, exists bool) kafkacaller.Metadata {
	now := time.Now().Unix()
	m := kafkacaller.Metadata{Initialized: true, Exists: exists, RequestID: id, Scenario: scenario, StreamID: streamTest, Variants: []kafkacaller.VariantMetadata{}}
	if exists {
		m.MoneyInstanceID = instanceTest
		m.CreatedAt = now
		m.Deadline = now + 120
	}
	return m
}
func positiveReceipt(id, digest string) ledger.Receipt {
	sum := sha256.Sum256([]byte("poc-spire-transfer\x00" + identity.Trusted.String() + "\x00" + id))
	now := time.Now().Unix()
	result := ledger.Result{RequestID: id, Caller: identity.Trusted.String(), Amount: 100, Currency: "USD", TransferID: hex.EncodeToString(sum[:]), Snapshot: ledger.Snapshot{Balances: map[string]int64{"demo-a": 99900, "demo-b": 100100}, Version: 1}}
	return ledger.Receipt{InstanceID: instanceTest, Digest: digest, ObservedAt: now, RetainUntil: now + 180, Disposition: "applied", Reason: "transfer_applied", VerifiedProducer: identity.Trusted.String(), RequestID: id, BusinessCode: 200, BusinessResult: &result}
}
func negativeReceipt(digest string) ledger.Receipt {
	now := time.Now().Unix()
	return ledger.Receipt{InstanceID: instanceTest, Digest: digest, ObservedAt: now, RetainUntil: now + 180, Disposition: "rejected", Reason: "invalid_signature"}
}
func metadataRead(context.Context) (ledger.KafkaMetadata, error) {
	return ledger.KafkaMetadata{InstanceID: instanceTest, StreamID: streamTest, ConsumerReady: true}, nil
}
func assertAttempt(t *testing.T, a *App, id string) *Attempt {
	t.Helper()
	a.Wait()
	a.mu.Lock()
	defer a.mu.Unlock()
	attempt := a.attempts[id]
	if attempt == nil {
		t.Fatal("missing attempt")
	}
	return attempt
}
func TestKafkaSuccessfulPublicationRequiresAuthenticatedReceipt(t *testing.T) {
	key := strings.Repeat("k", 32)
	id := RequestID(operator, key)
	digest := strings.Repeat("1", 64)
	published := 0
	queried := 0
	m := fixtureMetadata(id, "kafka-trusted", true)
	m.Variants = []kafkacaller.VariantMetadata{{Digest: digest, ExpiresAt: time.Now().Unix() + 30, Publication: "queued"}}
	deps := KafkaDependencies{Metadata: metadataRead, Lookup: func(context.Context, string, string) (kafkacaller.Metadata, int, error) {
		return fixtureMetadata(id, "kafka-trusted", false), 404, nil
	}, Publish: func(_ context.Context, scenario string, r kafkacaller.PublishRequest) (kafkacaller.Metadata, int, error) {
		published++
		if scenario != "kafka-trusted" || r.RequestID != id || r.MoneyInstanceID != instanceTest {
			t.Error("unbound fixed publication")
		}
		return m, 200, nil
	}, Receipt: func(_ context.Context, got string) (ledger.Receipt, ledger.KafkaMetadata, int, error) {
		queried++
		if got != digest {
			t.Error("wrong digest")
		}
		return positiveReceipt(id, digest), ledger.KafkaMetadata{InstanceID: instanceTest}, 200, nil
	}}
	a := newKafkaApp(t, context.Background(), Config{}, deps)
	cookie, csrf := signin(t, a, operator)
	w := request(a, "POST", "/api/kafka-attempts", kafkaBody(key, "kafka-trusted"), cookie, csrf)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	attempt := assertAttempt(t, a, id)
	if attempt.Status != "done" || attempt.Kafka.Outcome != "transfer_succeeded" || published != 1 || queried != 1 || attempt.Result != nil {
		t.Fatal("broker ack masqueraded as Money evidence", attempt)
	}
	if w = request(a, "POST", "/api/kafka-attempts", kafkaBody(key, "kafka-trusted"), cookie, csrf); w.Code != 202 || published != 1 {
		t.Fatal("completed retry redispatched")
	}
	if w = request(a, "POST", "/api/attempts", attemptBody(key, "trusted"), cookie, csrf); w.Code != 409 {
		t.Fatal("retained cross-scenario ID did not conflict")
	}
}
func TestKafkaUIRestartRecoversSameIDBeforeAnyPublication(t *testing.T) {
	key := strings.Repeat("r", 32)
	id := RequestID(operator, key)
	digest := strings.Repeat("2", 64)
	m := fixtureMetadata(id, "kafka-trusted", true)
	m.CreatedAt -= 20
	m.Deadline -= 20
	m.Variants = []kafkacaller.VariantMetadata{{Digest: digest, ExpiresAt: time.Now().Unix() + 10, Publication: "publish_unknown"}}
	for i := 0; i < 2; i++ {
		pubs := 0
		deps := KafkaDependencies{Metadata: metadataRead, Lookup: func(context.Context, string, string) (kafkacaller.Metadata, int, error) { return m, 200, nil }, Publish: func(context.Context, string, kafkacaller.PublishRequest) (kafkacaller.Metadata, int, error) {
			pubs++
			return m, 200, nil
		}, Receipt: func(context.Context, string) (ledger.Receipt, ledger.KafkaMetadata, int, error) {
			return positiveReceipt(id, digest), ledger.KafkaMetadata{InstanceID: instanceTest}, 200, nil
		}}
		a := newKafkaApp(t, context.Background(), Config{}, deps)
		cookie, csrf := signin(t, a, operator)
		if request(a, "POST", "/api/kafka-attempts", kafkaBody(key, "kafka-trusted"), cookie, csrf).Code != 202 {
			t.Fatal("creation")
		}
		attempt := assertAttempt(t, a, id)
		if pubs != 0 || attempt.Kafka.Outcome != "transfer_succeeded" || attempt.Kafka.Caller.CreatedAt != m.CreatedAt || attempt.Kafka.Caller.Deadline != m.Deadline {
			t.Fatal("restart republished or refreshed descriptor")
		}
	}
}
func TestAllVariantsSettleAndInvalidRecordsStayUnattributed(t *testing.T) {
	id := "one"
	d1, d2 := strings.Repeat("a", 64), strings.Repeat("b", 64)
	m := fixtureMetadata(id, "kafka-trusted", true)
	m.Variants = []kafkacaller.VariantMetadata{{Digest: d1}, {Digest: d2}}
	reject := negativeReceipt(d2)
	queries := []OutcomeQuery{{Digest: d1, HTTPStatus: 404}, {Digest: d2, HTTPStatus: 200, Receipt: &reject}}
	if outcome, err := settle(id, "kafka-trusted", instanceTest, m, queries, time.Now().Unix()); err != nil || outcome != "" {
		t.Fatal("latest rejection hid earlier unknown", outcome, err)
	}
	applied := positiveReceipt(id, d1)
	queries[0] = OutcomeQuery{Digest: d1, HTTPStatus: 200, Receipt: &applied}
	if outcome, err := settle(id, "kafka-trusted", instanceTest, m, queries, time.Now().Unix()); err != nil || outcome != "transfer_succeeded" {
		t.Fatal(outcome, err)
	}
	rejectFirst := negativeReceipt(d1)
	queries[0] = OutcomeQuery{Digest: d1, HTTPStatus: 200, Receipt: &rejectFirst}
	if outcome, err := settle(id, "kafka-tampered", instanceTest, m, queries, time.Now().Unix()); err != nil || outcome != "money_rejected" {
		t.Fatal(outcome, err)
	}
	reject.VerifiedProducer = identity.Trusted.String()
	if _, err := settle(id, "kafka-tampered", instanceTest, m, queries, time.Now().Unix()); err == nil {
		t.Fatal("invalid JWS claimed producer")
	}
	reject.VerifiedProducer = ""
	reject.RetainUntil = time.Now().Unix()
	if outcome, err := settle(id, "kafka-tampered", instanceTest, m, queries, time.Now().Unix()); err != nil || outcome != "" {
		t.Fatal("expired evidence settled rejection")
	}
}
func TestKafkaResetMissingStateAndUncorrelatedReceiptStopPublication(t *testing.T) {
	for _, mode := range []string{"reset", "state-loss", "mismatched-digest", "mismatched-instance", "wrong-id"} {
		t.Run(mode, func(t *testing.T) {
			key := strings.Repeat("x", 32)
			id := RequestID(operator, key)
			digest := strings.Repeat("3", 64)
			m := fixtureMetadata(id, "kafka-trusted", true)
			m.Variants = []kafkacaller.VariantMetadata{{Digest: digest, Publication: "publish_unknown"}}
			if mode == "reset" {
				m.MoneyInstanceID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			}
			if mode == "wrong-id" {
				m.RequestID = "other"
			}
			pubs := 0
			deps := KafkaDependencies{Metadata: metadataRead, Lookup: func(context.Context, string, string) (kafkacaller.Metadata, int, error) {
				if mode == "state-loss" {
					return kafkacaller.Metadata{}, 503, errors.New("missing_initialized_state")
				}
				return m, 200, nil
			}, Publish: func(context.Context, string, kafkacaller.PublishRequest) (kafkacaller.Metadata, int, error) {
				pubs++
				return m, 200, nil
			}, Receipt: func(context.Context, string) (ledger.Receipt, ledger.KafkaMetadata, int, error) {
				r := positiveReceipt(id, digest)
				if mode == "mismatched-digest" {
					r.Digest = strings.Repeat("4", 64)
				}
				instance := instanceTest
				if mode == "mismatched-instance" {
					instance = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
				}
				return r, ledger.KafkaMetadata{InstanceID: instance}, 200, nil
			}}
			a := newKafkaApp(t, context.Background(), Config{}, deps)
			cookie, csrf := signin(t, a, operator)
			if request(a, "POST", "/api/kafka-attempts", kafkaBody(key, "kafka-trusted"), cookie, csrf).Code != 202 {
				t.Fatal("creation")
			}
			attempt := assertAttempt(t, a, id)
			if pubs != 0 || attempt.Status != "retryable" || attempt.Kafka.Outcome != "unknown_commit" {
				t.Fatal("reset/invalid evidence caused effects or settled", pubs, attempt)
			}
		})
	}
}
func TestKafkaBrowserSchemaAndSecurityShareExistingBoundary(t *testing.T) {
	var pubs atomic.Int64
	deps := KafkaDependencies{Metadata: metadataRead, Lookup: func(context.Context, string, string) (kafkacaller.Metadata, int, error) {
		return kafkacaller.Metadata{}, 503, errors.New("stop")
	}, Publish: func(context.Context, string, kafkacaller.PublishRequest) (kafkacaller.Metadata, int, error) {
		pubs.Add(1)
		return kafkacaller.Metadata{}, 503, errors.New("stop")
	}, Receipt: func(context.Context, string) (ledger.Receipt, ledger.KafkaMetadata, int, error) {
		return ledger.Receipt{}, ledger.KafkaMetadata{}, 404, nil
	}}
	a := newKafkaApp(t, context.Background(), Config{}, deps)
	cookie, csrf := signin(t, a, operator)
	viewer, viewerCSRF := signin(t, a, Principal{Issuer: operator.Issuer, Subject: "viewer", Role: "viewer"})
	valid := kafkaBody(strings.Repeat("b", 32), "kafka-trusted")
	for _, c := range []struct {
		body   []byte
		cookie *http.Cookie
		csrf   string
		code   int
	}{{valid, nil, "", 401}, {valid, viewer, viewerCSRF, 403}, {valid, cookie, "", 403}, {[]byte(strings.Replace(string(valid), streamTest, strings.Repeat("f", 32), 1)), cookie, csrf, 400}, {[]byte(strings.Replace(string(valid), "kafka-trusted", "generic", 1)), cookie, csrf, 400}, {[]byte(strings.Replace(string(valid), `"scenario":`, `"amount_minor":100,"scenario":`, 1)), cookie, csrf, 400}, {append(append([]byte{}, valid...), []byte(" {}")...), cookie, csrf, 400}, {[]byte(strings.Repeat("x", 1025)), cookie, csrf, 400}} {
		if w := request(a, "POST", "/api/kafka-attempts", c.body, c.cookie, c.csrf); w.Code != c.code {
			t.Fatal(w.Code, c.code, string(c.body))
		}
	}
	if pubs.Load() != 0 || len(a.attempts) != 0 {
		t.Fatal("rejected browser boundary dispatched")
	}
	html := request(a, "GET", "/", nil, cookie, "").Body.String()
	for _, label := range []string{"Sync (HTTP/mTLS)", "Async (Kafka)", "kafka-trusted", "kafka-unidentified", "kafka-tampered"} {
		if !strings.Contains(html, label) {
			t.Fatal("missing fixed menu", label)
		}
	}
}
func TestKafkaAndSyncShareConcurrencyAndRateSlots(t *testing.T) {
	gate := make(chan struct{})
	started := make(chan struct{})
	deps := KafkaDependencies{Metadata: func(ctx context.Context) (ledger.KafkaMetadata, error) {
		close(started)
		select {
		case <-gate:
		case <-ctx.Done():
		}
		return ledger.KafkaMetadata{}, errors.New("stop")
	}, Lookup: func(context.Context, string, string) (kafkacaller.Metadata, int, error) {
		return kafkacaller.Metadata{}, 503, nil
	}, Publish: func(context.Context, string, kafkacaller.PublishRequest) (kafkacaller.Metadata, int, error) {
		return kafkacaller.Metadata{}, 503, nil
	}, Receipt: func(context.Context, string) (ledger.Receipt, ledger.KafkaMetadata, int, error) {
		return ledger.Receipt{}, ledger.KafkaMetadata{}, 404, nil
	}}
	a := newKafkaApp(t, context.Background(), Config{Concurrent: 1, Rate: 1}, deps)
	cookie, csrf := signin(t, a, operator)
	key := strings.Repeat("z", 32)
	if request(a, "POST", "/api/kafka-attempts", kafkaBody(key, "kafka-trusted"), cookie, csrf).Code != 202 {
		t.Fatal("creation")
	}
	<-started
	if request(a, "POST", "/api/attempts", attemptBody(strings.Repeat("q", 32), "trusted"), cookie, csrf).Code != 429 {
		t.Fatal("Sync bypassed shared concurrency")
	}
	close(gate)
	a.Wait()
	if request(a, "POST", "/api/attempts", attemptBody(strings.Repeat("q", 32), "trusted"), cookie, csrf).Code != 429 {
		t.Fatal("Sync bypassed shared rate")
	}
}

func TestEarlierQueryFailureCannotHideAnotherAuthenticatedAppliedVariant(t *testing.T) {
	key := strings.Repeat("v", 32)
	id := RequestID(operator, key)
	first, last := strings.Repeat("a", 64), strings.Repeat("b", 64)
	m := fixtureMetadata(id, "kafka-trusted", true)
	m.Variants = []kafkacaller.VariantMetadata{{Digest: first, Publication: "publish_unknown"}, {Digest: last, Publication: "queued"}}
	queries := 0
	pubs := 0
	deps := KafkaDependencies{Metadata: metadataRead, Lookup: func(context.Context, string, string) (kafkacaller.Metadata, int, error) { return m, 200, nil }, Publish: func(context.Context, string, kafkacaller.PublishRequest) (kafkacaller.Metadata, int, error) {
		pubs++
		return m, 200, nil
	}, Receipt: func(_ context.Context, digest string) (ledger.Receipt, ledger.KafkaMetadata, int, error) {
		queries++
		if digest == first {
			return ledger.Receipt{}, ledger.KafkaMetadata{}, 0, errors.New("timeout")
		}
		return positiveReceipt(id, last), ledger.KafkaMetadata{InstanceID: instanceTest}, 200, nil
	}}
	a := newKafkaApp(t, context.Background(), Config{}, deps)
	cookie, csrf := signin(t, a, operator)
	if request(a, "POST", "/api/kafka-attempts", kafkaBody(key, "kafka-trusted"), cookie, csrf).Code != 202 {
		t.Fatal("creation")
	}
	attempt := assertAttempt(t, a, id)
	if queries != 2 || pubs != 0 || attempt.Status != "done" || attempt.Kafka.Outcome != "transfer_succeeded" {
		t.Fatal("available applied proof hidden by another failure", queries, pubs, attempt)
	}
}
func TestExpiredDescriptorReadsMissingEvidenceWithoutRepublication(t *testing.T) {
	key := strings.Repeat("e", 32)
	id := RequestID(operator, key)
	digest := strings.Repeat("d", 64)
	m := fixtureMetadata(id, "kafka-trusted", true)
	m.CreatedAt -= 121
	m.Deadline -= 121
	m.Variants = []kafkacaller.VariantMetadata{{Digest: digest, Publication: "publish_unknown"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queries := 0
	pubs := 0
	deps := KafkaDependencies{Metadata: metadataRead, Lookup: func(context.Context, string, string) (kafkacaller.Metadata, int, error) { return m, 200, nil }, Publish: func(context.Context, string, kafkacaller.PublishRequest) (kafkacaller.Metadata, int, error) {
		pubs++
		return m, 200, nil
	}, Receipt: func(context.Context, string) (ledger.Receipt, ledger.KafkaMetadata, int, error) {
		queries++
		cancel()
		return ledger.Receipt{}, ledger.KafkaMetadata{InstanceID: instanceTest, StreamID: streamTest}, 404, nil
	}}
	a := newKafkaApp(t, ctx, Config{}, deps)
	cookie, csrf := signin(t, a, operator)
	if request(a, "POST", "/api/kafka-attempts", kafkaBody(key, "kafka-trusted"), cookie, csrf).Code != 202 {
		t.Fatal("creation")
	}
	attempt := assertAttempt(t, a, id)
	if queries != 1 || pubs != 0 || attempt.Status != "retryable" || attempt.Kafka.Outcome != "unknown_commit" || attempt.Kafka.Caller.Deadline != m.Deadline {
		t.Fatal("expired descriptor refreshed or evidence absence implied rejection")
	}
}
