package control

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/kafkacmd"
)

func TestNewKafkaRoutesKeepPeerBoundaryAndOldSyncSchema(t *testing.T) {
	policy, _ := kafkacmd.NewPolicy("0123456789abcdef0123456789abcdef")
	dir := filepath.Join(t.TempDir(), "caller")
	if err := kafkacaller.Initialize(dir, policy, "unidentified"); err != nil {
		t.Fatal(err)
	}
	store, err := kafkacaller.Open(dir, policy, "unidentified")
	if err != nil {
		t.Fatal(err)
	}
	published := 0
	synced := 0
	sender := &kafkacaller.Sender{Store: store, NoIdentity: func(context.Context) error { return nil }, Publish: func(context.Context, kafkacmd.Record) error { published++; return nil }}
	handler := endpoint(1000, func(_ context.Context, id string) Result {
		synced++
		return Result{RequestID: id, Outcome: "tls_rejected"}
	}, kafkacaller.Handler(sender))
	valid := `{"request_id":"one","scenario":"kafka-unidentified","money_instance_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	for _, c := range []struct {
		method, path, body string
		uid                uint32
		code               int
	}{
		{"POST", "/attempt/kafka", valid, 1001, 403},
		{"POST", "/attempt/kafka", strings.Replace(valid, "kafka-unidentified", "kafka-trusted", 1), 1000, 400},
		{"POST", "/attempt/kafka", strings.Replace(valid, `"one"`, `"bad space"`, 1), 1000, 400},
		{"POST", "/attempt/kafka", strings.Replace(valid, `"one"`, `"one","amount_minor":100`, 1), 1000, 400},
		{"POST", "/attempt/kafka", strings.Replace(valid, `"one"`, `"one","\u0072equest_id":"two"`, 1), 1000, 400},
		{"POST", "/attempt/kafka", strings.Repeat("x", 1025), 1000, 400},
		{"POST", "/attempt/kafka", valid + " {}", 1000, 400},
		{"POST", "/attempt", valid, 1000, 400},
		{"POST", "/attempt", `{"request_id":"sync"}`, 1000, 200},
		{"GET", "/attempt/kafka/one", "", 1000, 404},
		{"POST", "/attempt/kafka", valid, 1000, 200},
		{"GET", "/attempt/kafka/one", "", 1000, 200},
		{"GET", "/attempt/kafka/one", "{}", 1000, 400},
		{"GET", "/attempt/kafka/one?x=1", "", 1000, 400},
		{"GET", "/attempt/kafka/one", "", 1001, 403},
	} {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		req = req.WithContext(context.WithValue(req.Context(), uidKey{}, c.uid))
		reply := httptest.NewRecorder()
		handler.ServeHTTP(reply, req)
		if reply.Code != c.code {
			t.Fatal(c.path, c.body, reply.Code, c.code)
		}
	}
	if published != 1 || synced != 1 {
		t.Fatal("rejected routes dispatched", published, synced)
	}
}
