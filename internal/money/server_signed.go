package money

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"poc-spire/internal/httpjws"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/ledger"
)

type signedHandler struct {
	ledger *ledger.Ledger
	audit  io.Writer
	signer x509svid.Source
	bundle x509bundle.Source
	now    func() time.Time
	mu     sync.Mutex
}

func (h *signedHandler) clock() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

func (h *signedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/jose")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "close")
	raw, err := io.ReadAll(io.LimitReader(r.Body, httpjws.MaxValue+1))
	if err != nil || len(raw) > httpjws.MaxValue {
		h.finish(w, r, 400, map[string]string{"error": "malformed_jws"}, "unsigned", "")
		return
	}
	if len(raw) == 0 {
		h.finish(w, r, 401, map[string]string{"error": "invalid_signature"}, "unsigned", "")
		return
	}
	verified, fail := httpjws.Verify(string(raw), h.bundle, h.clock())
	if fail != nil {
		requestID := "unsigned"
		if verified != nil && verified.RequestID != "" {
			requestID = verified.RequestID
		}
		h.finish(w, r, fail.Status, map[string]string{"error": fail.Reason}, requestID, "")
		return
	}
	if verified.Method != r.Method || verified.Path != r.URL.Path {
		h.finish(w, r, 401, map[string]string{"error": "destination_mismatch"}, verified.RequestID, verified.Signer.String())
		return
	}
	if !verified.Current(h.clock()) {
		h.finish(w, r, 403, map[string]string{"error": "credential_not_current"}, verified.RequestID, verified.Signer.String())
		return
	}
	id := verified.Signer.String()
	metadata, kafkaEnabled := h.ledger.KafkaMetadata()
	kafkaRoute := kafkaEnabled && r.Method == "GET" && id == identity.Observer.String() && (r.URL.Path == "/kafka/outcomes" || strings.HasPrefix(r.URL.Path, "/kafka/outcomes/") && kafkacmd.Hex64.MatchString(strings.TrimPrefix(r.URL.Path, "/kafka/outcomes/")))
	if !permitted(r.Method, r.URL.Path, id) && !kafkaRoute {
		h.finish(w, r, 403, map[string]string{"error": "forbidden"}, verified.RequestID, id)
		return
	}
	if kafkaRoute || r.URL.Path == "/balances" {
		if string(verified.Body) != "null" || r.URL.RawQuery != "" {
			h.finish(w, r, 400, map[string]string{"error": "invalid_bounded_json"}, verified.RequestID, id)
			return
		}
		if !verified.Current(h.clock()) {
			h.finish(w, r, 403, map[string]string{"error": "credential_not_current"}, verified.RequestID, id)
			return
		}
		if r.URL.Path == "/balances" {
			h.finish(w, r, 200, h.ledger.Snapshot(), verified.RequestID, id)
			return
		}
		if r.URL.Path == "/kafka/outcomes" {
			h.finish(w, r, 200, metadata, verified.RequestID, id)
			return
		}
		digest := strings.TrimPrefix(r.URL.Path, "/kafka/outcomes/")
		receipt, ok := h.ledger.KafkaReceipt(digest)
		if !ok {
			h.finish(w, r, 404, metadata, verified.RequestID, id)
			return
		}
		h.finish(w, r, 200, receipt, verified.RequestID, id)
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(verified.Body))
	decoder.DisallowUnknownFields()
	var request ledger.Request
	if err := decoder.Decode(&request); err != nil || request.RequestID != verified.RequestID {
		h.finish(w, r, 400, map[string]string{"error": "invalid_bounded_json"}, verified.RequestID, id)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		h.finish(w, r, 400, map[string]string{"error": "invalid_bounded_json"}, verified.RequestID, id)
		return
	}
	if !verified.Current(h.clock()) {
		h.finish(w, r, 403, map[string]string{"error": "credential_not_current"}, verified.RequestID, id)
		return
	}
	result, status := h.ledger.Transfer(id, request)
	h.finish(w, r, status, result, verified.RequestID, id)
}

func (h *signedHandler) finish(w http.ResponseWriter, r *http.Request, status int, value any, requestID, signer string) {
	if !kafkacmd.RequestID.MatchString(requestID) {
		requestID = "unsigned"
	}
	raw, err := json.Marshal(value)
	svid, svidErr := h.signer.GetX509SVID()
	var compact string
	if err == nil && svidErr == nil && svid != nil && svid.ID == identity.Money {
		compact, err = httpjws.Sign(svid, r.Method, r.URL.Path, requestID, raw, h.clock())
	}
	if err != nil || compact == "" {
		status = 503
		compact = ""
	}
	h.mu.Lock()
	observation := map[string]any{"event": "money_request", "verified_peer": signer, "method": r.Method, "path": r.URL.Path, "status": status,
		"at_utc": h.clock().UTC().Format(time.RFC3339Nano), "remote": r.RemoteAddr, "transport": "signed-http"}
	if local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		observation["local"] = local.String()
	}
	json.NewEncoder(h.audit).Encode(observation)
	h.mu.Unlock()
	if compact == "" {
		w.WriteHeader(status)
		return
	}
	w.WriteHeader(status)
	w.Write([]byte(compact))
}

func SignedServer(source *workloadapi.X509Source, book *ledger.Ledger, audit io.Writer) *http.Server {
	if audit == nil {
		audit = io.Discard
	}
	server := &http.Server{Handler: &signedHandler{ledger: book, audit: audit, signer: source, bundle: source},
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){}, ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout: 5 * time.Second, WriteTimeout: 8 * time.Second, IdleTimeout: time.Second, MaxHeaderBytes: 8192,
		ErrorLog: log.New(audit, "money_http: ", 0)}
	server.SetKeepAlivesEnabled(false)
	return server
}
