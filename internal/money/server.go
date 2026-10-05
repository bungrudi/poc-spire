package money

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

func permitted(method, path, id string) bool {
	return method == "POST" && path == "/transfer" && id == identity.Trusted.String() ||
		method == "GET" && path == "/balances" && id == identity.Observer.String()
}

type handler struct {
	ledger *ledger.Ledger
	audit  io.Writer
	mu     sync.Mutex
}

func credentialCurrent(r *http.Request) bool {
	if r.TLS == nil || !r.TLS.HandshakeComplete || len(r.TLS.PeerCertificates) == 0 {
		return false
	}
	cert := r.TLS.PeerCertificates[0]
	now := time.Now()
	return !now.Before(cert.NotBefore) && now.Before(cert.NotAfter)
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "close")
	id := ""
	if r.TLS != nil && r.TLS.HandshakeComplete && len(r.TLS.PeerCertificates) > 0 {
		if peer, err := x509svid.IDFromCert(r.TLS.PeerCertificates[0]); err == nil {
			id = peer.String()
		}
	}
	write := func(status int, value any) {
		h.mu.Lock()
		json.NewEncoder(h.audit).Encode(map[string]any{"event": "money_request", "verified_peer": id, "method": r.Method, "path": r.URL.Path, "status": status})
		h.mu.Unlock()
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(value)
	}
	if !credentialCurrent(r) {
		write(403, map[string]string{"error": "credential_not_current"})
		return
	}
	if !permitted(r.Method, r.URL.Path, id) {
		write(403, map[string]string{"error": "forbidden"})
		return
	}
	if r.URL.Path == "/balances" {
		write(200, h.ledger.Snapshot())
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request ledger.Request
	if err := decoder.Decode(&request); err != nil {
		write(400, map[string]string{"error": "invalid_bounded_json"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		write(400, map[string]string{"error": "invalid_bounded_json"})
		return
	}
	// Reading a bounded body can cross the peer credential expiry boundary.
	if !credentialCurrent(r) {
		write(403, map[string]string{"error": "credential_not_current"})
		return
	}
	result, status := h.ledger.Transfer(id, request)
	write(status, result)
}

func Server(source *workloadapi.X509Source, book *ledger.Ledger, audit io.Writer) *http.Server {
	if audit == nil {
		audit = io.Discard
	}
	server := &http.Server{Handler: &handler{ledger: book, audit: audit}, TLSConfig: identity.ServerTLS(source, source),
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){}, ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout: 5 * time.Second, WriteTimeout: 8 * time.Second, IdleTimeout: time.Second, MaxHeaderBytes: 8192,
		ErrorLog: log.New(audit, "money_tls: ", 0)}
	server.SetKeepAlivesEnabled(false)
	return server
}
