package kafkacaller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/kafkaio"
)

type PublishRequest struct {
	RequestID       string `json:"request_id"`
	Scenario        string `json:"scenario"`
	MoneyInstanceID string `json:"money_instance_id"`
}
type ControlError struct {
	Error    string   `json:"error"`
	Metadata Metadata `json:"metadata"`
}

// Handler is mounted only behind control's kernel peer-UID check and shared
// concurrency limiter on the existing private caller Unix socket.
func Handler(sender *Sender) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		write := func(code int, m Metadata, err error) {
			w.WriteHeader(code)
			if err != nil {
				json.NewEncoder(w).Encode(ControlError{err.Error(), m})
			} else {
				json.NewEncoder(w).Encode(m)
			}
		}
		if r.URL.RawQuery != "" {
			write(400, Metadata{}, errors.New("unsupported_control_query"))
			return
		}
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/attempt/kafka/") {
			raw, err := io.ReadAll(io.LimitReader(r.Body, 1))
			if err != nil || len(raw) != 0 {
				write(400, Metadata{}, errors.New("unsupported_lookup_body"))
				return
			}
			id := strings.TrimPrefix(r.URL.Path, "/attempt/kafka/")
			m, code, err := sender.Store.Lookup(id)
			write(code, m, err)
			return
		}
		if r.Method != "POST" || r.URL.Path != "/attempt/kafka" {
			write(404, Metadata{}, errors.New("unsupported_command"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			write(400, Metadata{}, errors.New("invalid_bounded_command"))
			return
		}
		obj, err := kafkacmd.StrictJSON(raw)
		valid := err == nil && len(obj) == 3
		for _, name := range []string{"request_id", "scenario", "money_instance_id"} {
			if _, ok := obj[name].(string); !ok {
				valid = false
			}
		}
		var request PublishRequest
		if !valid || json.Unmarshal(raw, &request) != nil || !kafkacmd.RequestID.MatchString(request.RequestID) || !kafkacmd.Hex32.MatchString(request.MoneyInstanceID) || !allowed(sender.Store.role, request.Scenario) {
			write(400, Metadata{}, errors.New("invalid_fixed_command"))
			return
		}
		m, code, err := sender.Send(r.Context(), request.RequestID, request.Scenario, request.MoneyInstanceID)
		write(code, m, err)
	})
}
func New(configPath, role, workloadSocket string, source x509svid.Source) (*Sender, func(), error) {
	cfg, err := kafkaio.LoadConfig(configPath)
	if err != nil {
		return nil, nil, err
	}
	store, err := Configure(cfg.StateDir, cfg.Policy(), role)
	if err != nil {
		return nil, nil, err
	}
	client, err := kafkaio.Producer(cfg)
	if err != nil {
		return nil, nil, err
	}
	sender := &Sender{Store: store, Source: source, Publish: func(ctx context.Context, r kafkacmd.Record) error { return kafkaio.Publish(ctx, client, r) }}
	if role == "unidentified" {
		sender.NoIdentity = NoIdentity(workloadSocket)
	}
	return sender, client.Close, nil
}

// Private calls return diagnostics only. They never report a Money disposition.
func Call(ctx context.Context, socket string, request *PublishRequest, id string) (Metadata, int, error) {
	transport := &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return CallHTTP(ctx, client, "http://unix", request, id)
}

// CallHTTP preserves lookup/publication deadlines and all private metadata correlation checks.
func CallHTTP(ctx context.Context, client *http.Client, origin string, request *PublishRequest, id string) (Metadata, int, error) {
	timeout := 2 * time.Second
	if request != nil {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if request != nil && request.RequestID != id {
		return Metadata{}, 400, errors.New("mismatched_control_id")
	}
	method, path := http.MethodGet, "/attempt/kafka/"+id
	var body io.Reader
	if request != nil {
		method, path = http.MethodPost, "/attempt/kafka"
		raw, _ := json.Marshal(request)
		body = bytes.NewReader(raw)
	}
	if !kafkacmd.RequestID.MatchString(id) {
		return Metadata{}, 400, errors.New("invalid_request_id")
	}
	req, err := http.NewRequestWithContext(ctx, method, origin+path, body)
	if err != nil {
		return Metadata{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	reply, err := client.Do(req)
	if err != nil {
		return Metadata{}, 0, errors.New("caller_control_unknown")
	}
	defer reply.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(reply.Body, 8193))
	if err != nil || len(raw) > 8192 {
		return Metadata{}, reply.StatusCode, errors.New("caller_control_unknown")
	}
	if reply.StatusCode != 200 && reply.StatusCode != 404 {
		return Metadata{}, reply.StatusCode, errors.New("caller_control_unavailable")
	}
	var m Metadata
	if strictDecode(raw, &m) != nil || m.RequestID != id || !m.Initialized || !kafkacmd.Hex32.MatchString(m.StreamID) || len(m.Variants) > 4 {
		return Metadata{}, reply.StatusCode, errors.New("caller_metadata_uncorrelated")
	}
	if reply.StatusCode == 404 && (m.Exists || len(m.Variants) != 0 || m.MoneyInstanceID != "" || m.Deadline != 0) {
		return Metadata{}, reply.StatusCode, errors.New("caller_metadata_uncorrelated")
	}
	if reply.StatusCode == 200 && (!m.Exists || !kafkacmd.Hex32.MatchString(m.MoneyInstanceID) || m.Deadline <= m.CreatedAt || m.Deadline-m.CreatedAt > 120) {
		return Metadata{}, reply.StatusCode, errors.New("caller_metadata_uncorrelated")
	}
	for _, v := range m.Variants {
		if !kafkacmd.Hex64.MatchString(v.Digest) || (v.Publication != "publish_unknown" && v.Publication != "queued") {
			return Metadata{}, reply.StatusCode, errors.New("caller_metadata_uncorrelated")
		}
	}
	return m, reply.StatusCode, nil
}
