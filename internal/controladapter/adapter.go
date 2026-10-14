// Package controladapter implements the opt-in exact-identity Kubernetes control hop.
package controladapter

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"poc-spire/internal/control"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/kafkacmd"
)

func ID(role string) (spiffeid.ID, error) {
	if role != "trusted" && role != "unidentified" {
		return spiffeid.ID{}, errors.New("invalid control role")
	}
	return spiffeid.FromString("spiffe://poc.internal/control/caller/" + role)
}

type Config struct {
	Role     string
	Socket   string
	RunValid func() bool
	Now      func() time.Time
}

type verifiedSignature struct {
	id      spiffeid.ID
	current func(time.Time) bool
}
type signatureKey struct{}

func current(r *http.Request, now time.Time) bool {
	if sig, ok := r.Context().Value(signatureKey{}).(verifiedSignature); ok {
		return sig.current(now)
	}
	if r.TLS == nil || !r.TLS.HandshakeComplete || len(r.TLS.PeerCertificates) == 0 {
		return false
	}
	for _, cert := range r.TLS.PeerCertificates {
		if now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
			return false
		}
	}
	return true
}

func Handler(cfg Config) http.Handler {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	limit := make(chan struct{}, 2)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "close")
		fail := func(status int, message string) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": message})
		}
		if !current(r, cfg.Now()) {
			fail(403, "credential_not_current")
			return
		}
		var peer spiffeid.ID
		var err error
		if sig, ok := r.Context().Value(signatureKey{}).(verifiedSignature); ok {
			peer = sig.id
		} else if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			peer, err = x509svid.IDFromCert(r.TLS.PeerCertificates[0])
		}
		if err != nil || peer != identity.Observer {
			fail(403, "forbidden")
			return
		}
		if _, err := ID(cfg.Role); err != nil || cfg.Socket == "" {
			fail(503, "control_unavailable")
			return
		}
		if cfg.RunValid == nil || !cfg.RunValid() {
			fail(503, "run_invalid")
			return
		}
		if r.URL.RawQuery != "" {
			fail(400, "unsupported_query")
			return
		}
		timeout := 10 * time.Second
		lookup := r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/attempt/kafka/")
		if lookup {
			timeout = 2 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		select {
		case limit <- struct{}{}:
			defer func() { <-limit }()
		default:
			fail(429, "caller_busy")
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1025))
		if err != nil || len(raw) > 1024 {
			fail(400, "invalid_bounded_command")
			return
		}
		var id string
		var publish *kafkacaller.PublishRequest
		if lookup {
			id = strings.TrimPrefix(r.URL.Path, "/attempt/kafka/")
			if len(raw) != 0 || !kafkacmd.RequestID.MatchString(id) {
				fail(400, "invalid_lookup")
				return
			}
		} else {
			if r.Method != "POST" || (r.URL.Path != "/attempt" && r.URL.Path != "/attempt/kafka") {
				fail(404, "unsupported_command")
				return
			}
			fields, err := kafkacmd.StrictJSON(raw)
			if err != nil {
				fail(400, "invalid_fixed_command")
				return
			}
			id, _ = fields["request_id"].(string)
			if !kafkacmd.RequestID.MatchString(id) {
				fail(400, "invalid_request_id")
				return
			}
			if r.URL.Path == "/attempt" {
				if len(fields) != 1 {
					fail(400, "invalid_fixed_command")
					return
				}
			} else {
				valid := len(fields) == 3
				for _, field := range []string{"request_id", "scenario", "money_instance_id"} {
					if _, ok := fields[field].(string); !ok {
						valid = false
					}
				}
				var request kafkacaller.PublishRequest
				if !valid || json.Unmarshal(raw, &request) != nil || !kafkacmd.Hex32.MatchString(request.MoneyInstanceID) {
					fail(400, "invalid_fixed_command")
					return
				}
				allowed := cfg.Role == "trusted" && (request.Scenario == "kafka-trusted" || request.Scenario == "kafka-tampered") || cfg.Role == "unidentified" && request.Scenario == "kafka-unidentified"
				if !allowed {
					fail(400, "invalid_fixed_scenario")
					return
				}
				publish = &request
			}
		}
		// Both expiry and destructive reset can be detected while the body is read.
		if !current(r, cfg.Now()) {
			fail(403, "credential_not_current")
			return
		}
		if cfg.RunValid == nil || !cfg.RunValid() {
			fail(503, "run_invalid")
			return
		}
		if r.URL.Path == "/attempt" {
			result := control.Call(ctx, cfg.Socket, id)
			json.NewEncoder(w).Encode(result)
			return
		}
		metadata, status, err := kafkacaller.Call(ctx, cfg.Socket, publish, id)
		if err != nil {
			fail(503, "caller_control_unknown")
			return
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(metadata)
	})
}

// observationResponse records only the response status; it does not inspect or retain bodies.
type observationResponse struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *observationResponse) WriteHeader(status int) {
	if !w.wrote {
		w.status = status
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *observationResponse) Write(body []byte) (int, error) {
	if !w.wrote {
		w.status = http.StatusOK
		w.wrote = true
	}
	return w.ResponseWriter.Write(body)
}

func observe(next http.Handler, role string, audit io.Writer) http.Handler {
	var mutex sync.Mutex
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &observationResponse{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(response, r)
		observation := map[string]any{"event": "control_request", "role": role, "at_utc": time.Now().UTC().Format(time.RFC3339Nano),
			"remote": r.RemoteAddr, "method": r.Method, "status": response.status}
		// Fixed route labels avoid exporting unsupported URLs or arbitrary input.
		route := "unsupported"
		switch {
		case r.URL.Path == "/attempt":
			route = "/attempt"
		case r.URL.Path == "/attempt/kafka":
			route = "/attempt/kafka"
		case strings.HasPrefix(r.URL.Path, "/attempt/kafka/"):
			route = "/attempt/kafka/{id}"
		}
		observation["path"] = route
		if local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
			observation["local"] = local.String()
		}
		if sig, ok := r.Context().Value(signatureKey{}).(verifiedSignature); ok {
			observation["verified_peer"] = sig.id.String()
			observation["transport"] = "signed-http"
		}
		if r.TLS != nil && r.TLS.HandshakeComplete {
			observation["tls_version"] = tls.VersionName(r.TLS.Version)
			observation["tls_session_resumed"] = r.TLS.DidResume
			if len(r.TLS.PeerCertificates) > 0 {
				if peer, err := x509svid.IDFromCert(r.TLS.PeerCertificates[0]); err == nil {
					observation["verified_peer"] = peer.String()
				}
			}
		}
		mutex.Lock()
		defer mutex.Unlock()
		json.NewEncoder(audit).Encode(observation)
	})
}

func Server(source *workloadapi.X509Source, cfg Config, audit io.Writer) *http.Server {
	if audit == nil {
		audit = io.Discard
	}
	server := &http.Server{Handler: observe(Handler(cfg), cfg.Role, audit), TLSConfig: identity.ServerTLS(source, source),
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){}, ReadHeaderTimeout: time.Second,
		ReadTimeout: 2 * time.Second, WriteTimeout: 12 * time.Second, MaxHeaderBytes: 2048,
		ErrorLog: log.New(audit, "control_tls: ", 0)}
	server.SetKeepAlivesEnabled(false)
	return server
}

func Client(source *workloadapi.X509Source, role string) (*http.Client, *http.Transport, error) {
	server, err := ID(role)
	if err != nil {
		return nil, nil, err
	}
	client, transport := identity.HTTPClient(identity.ClientTLS(source, source, server))
	client.Timeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 10 * time.Second
	return client, transport, nil
}
