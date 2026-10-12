package controladapter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"poc-spire/internal/control"
	"poc-spire/internal/identity"
)

func request(peer, body string, now time.Time) *http.Request {
	r := httptest.NewRequest("POST", "/attempt", strings.NewReader(body))
	u, _ := url.Parse(peer)
	r.TLS = &tls.ConnectionState{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{{URIs: []*url.URL{u}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute)}}}
	return r
}

func TestPeerFixedSchemaAndInvalidRunNeverReachCaller(t *testing.T) {
	now := time.Now()
	handler := Handler(Config{Role: "trusted", Socket: "/missing", RunValid: func() bool { return true }, Now: func() time.Time { return now }})
	for _, test := range []struct {
		peer, body string
		status     int
	}{
		{identity.Trusted.String(), `{"request_id":"one"}`, 403},
		{identity.Observer.String(), `{"request_id":"one","destination":"evil"}`, 400},
		{identity.Observer.String(), `{"request_id":"one","request_id":"two"}`, 400},
		{identity.Observer.String(), strings.Repeat("x", 1025), 400},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, request(test.peer, test.body, now))
		if w.Code != test.status {
			t.Fatalf("got %d want %d", w.Code, test.status)
		}
	}
	handler = Handler(Config{Role: "trusted", Socket: "/missing", RunValid: func() bool { return false }})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request(identity.Observer.String(), `{"request_id":"one"}`, now))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "run_invalid") {
		t.Fatal(w.Code, w.Body.String())
	}
}

type crossingReader struct {
	now  *time.Time
	next time.Time
	data *strings.Reader
}

func (b crossingReader) Read(p []byte) (int, error) { *b.now = b.next; return b.data.Read(p) }
func (b crossingReader) Close() error               { return nil }

func TestExpiryAndRunLossWhileReadingBodyDenyForwarding(t *testing.T) {
	now := time.Now()
	initial := now
	r := request(identity.Observer.String(), `{"request_id":"same"}`, now)
	r.Body = crossingReader{&now, now.Add(2 * time.Minute), strings.NewReader(`{"request_id":"same"}`)}
	handler := Handler(Config{Role: "trusted", Socket: "/missing", RunValid: func() bool { return true }, Now: func() time.Time { return now }})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "credential_not_current") {
		t.Fatal(w.Code, w.Body.String())
	}
	now = initial
	valid := true
	r = request(identity.Observer.String(), `{"request_id":"same"}`, now)
	r.Body = resetReader{&valid, strings.NewReader(`{"request_id":"same"}`)}
	handler = Handler(Config{Role: "trusted", Socket: "/missing", RunValid: func() bool { return valid }, Now: func() time.Time { return now }})
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "run_invalid") {
		t.Fatal(w.Code, w.Body.String())
	}
}

type resetReader struct {
	valid *bool
	data  *strings.Reader
}

func (b resetReader) Read(p []byte) (int, error) { *b.valid = false; return b.data.Read(p) }
func (b resetReader) Close() error               { return nil }

func TestFixedForwardingAndUncorrelatedResponseStayUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "caller.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "POST" || r.URL.Path != "/attempt" {
			t.Error("wrong route")
		}
		raw, _ := io.ReadAll(r.Body)
		var req control.Request
		json.Unmarshal(raw, &req)
		if req.RequestID != "same" {
			t.Error("request changed")
		}
		json.NewEncoder(w).Encode(control.Result{RequestID: "different", Stage: "money_http", Outcome: "transfer_succeeded", MoneyStatus: 200})
	})}
	go server.Serve(listener)
	defer server.Shutdown(context.Background())
	handler := Handler(Config{Role: "trusted", Socket: path, RunValid: func() bool { return true }})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request(identity.Observer.String(), `{"request_id":"same"}`, time.Now()))
	var result control.Result
	if json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal(w.Body.String())
	}
	if calls.Load() != 1 || result.RequestID != "same" || result.Outcome != "unknown_commit" || result.MoneyStatus != 0 {
		t.Fatal(result, calls.Load())
	}
}

func TestKafkaRejectsCaseAliasesWrongKeysAndTypesBeforeUDS(t *testing.T) {
	handler := Handler(Config{Role: "trusted", Socket: "/missing", RunValid: func() bool { return true }})
	for _, body := range []string{
		`{"request_id":"one","SCENARIO":"kafka-trusted","MONEY_INSTANCE_ID":"0123456789abcdef0123456789abcdef"}`,
		`{"request_id":"one","scenario":"kafka-trusted","destination":"0123456789abcdef0123456789abcdef"}`,
		`{"request_id":"one","scenario":12,"money_instance_id":"0123456789abcdef0123456789abcdef"}`,
		`{"request_id":"one","scenario":"kafka-unidentified","money_instance_id":"0123456789abcdef0123456789abcdef"}`,
	} {
		req := request(identity.Observer.String(), body, time.Now())
		req.URL.Path = "/attempt/kafka"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
