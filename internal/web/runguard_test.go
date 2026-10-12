package web

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"poc-spire/internal/control"
	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/ledger"
)

func TestInvalidKubernetesRunStopsSyncKafkaCreationAndRetry(t *testing.T) {
	var calls atomic.Int32
	a := newKafkaApp(t, context.Background(), Config{}, KafkaDependencies{Metadata: metadataRead,
		Lookup: func(context.Context, string, string) (kafkacaller.Metadata, int, error) {
			t.Fatal("invalid caller lookup")
			return kafkacaller.Metadata{}, 0, nil
		},
		Publish: func(context.Context, string, kafkacaller.PublishRequest) (kafkacaller.Metadata, int, error) {
			t.Fatal("invalid caller publication")
			return kafkacaller.Metadata{}, 0, nil
		},
		Receipt: func(context.Context, string) (ledger.Receipt, ledger.KafkaMetadata, int, error) {
			return ledger.Receipt{}, ledger.KafkaMetadata{}, 404, nil
		},
	})
	valid := true
	a.deps.RunValid = func() bool { return valid }
	a.deps.Execute = func(_ context.Context, _ string, id string) control.Result {
		calls.Add(1)
		return control.Result{RequestID: id, Outcome: "unknown_commit"}
	}
	cookie, csrf := signin(t, a, operator)
	key := strings.Repeat("g", 32)
	if request(a, "POST", "/api/attempts", attemptBody(key, "trusted"), cookie, csrf).Code != 202 {
		t.Fatal("initial creation")
	}
	id := RequestID(operator, key)
	old := assertAttempt(t, a, id)
	created := old.Created
	valid = false
	for _, probe := range []struct {
		path string
		body []byte
	}{
		{"/api/attempts", attemptBody(key, "trusted")},
		{"/api/attempts", attemptBody(strings.Repeat("h", 32), "unidentified")},
		{"/api/kafka-attempts", kafkaBody(strings.Repeat("i", 32), "kafka-trusted")},
	} {
		w := request(a, "POST", probe.path, probe.body, cookie, csrf)
		if w.Code != 503 || !strings.Contains(w.Body.String(), "run_invalid") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if calls.Load() != 1 || len(a.attempts) != 1 || a.attempts[id].Created != created || a.attempts[id].Status != "retryable" {
		t.Fatal("invalid dispatch changed retry state")
	}
	if request(a, "GET", "/api/attempts/"+id, nil, cookie, "").Code != 200 {
		t.Fatal("historical status unavailable")
	}
}
