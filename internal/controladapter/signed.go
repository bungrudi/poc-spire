package controladapter

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"poc-spire/internal/httpjws"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacmd"
)

type bodyCapture struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
	wrote  bool
}

func (b *bodyCapture) WriteHeader(status int) {
	if !b.wrote {
		b.status = status
		b.wrote = true
	}
}
func (b *bodyCapture) Write(p []byte) (int, error) {
	if !b.wrote {
		b.status = http.StatusOK
		b.wrote = true
	}
	return b.body.Write(p)
}

func writeSigned(w http.ResponseWriter, r *http.Request, source identity.Material, status int, body []byte, requestID string) {
	if !kafkacmd.RequestID.MatchString(requestID) {
		requestID = "unsigned"
	}
	if len(body) == 0 {
		body = []byte(`{"error":"empty_response"}`)
	}
	svid, err := source.GetX509SVID()
	compact := ""
	if err == nil && svid != nil {
		compact, err = httpjws.Sign(svid, r.Method, r.URL.Path, requestID, body, time.Now())
	}
	if err != nil || compact == "" {
		status = 503
		compact = ""
	}
	w.Header().Set("Content-Type", "application/jose")
	w.WriteHeader(status)
	if compact != "" {
		w.Write([]byte(compact))
	}
}

func SignedHandler(source identity.Material, cfg Config) http.Handler {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	inner := Handler(cfg)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(io.LimitReader(r.Body, httpjws.MaxValue+1))
		if err != nil || len(raw) > httpjws.MaxValue {
			writeSigned(w, r, source, 400, []byte(`{"error":"malformed_jws"}`), "unsigned")
			return
		}
		if len(raw) == 0 {
			writeSigned(w, r, source, 401, []byte(`{"error":"invalid_signature"}`), "unsigned")
			return
		}
		verified, fail := httpjws.Verify(string(raw), source, cfg.Now())
		if fail != nil {
			requestID := "unsigned"
			if verified != nil {
				requestID = verified.RequestID
			}
			writeSigned(w, r, source, fail.Status, []byte(`{"error":"`+fail.Reason+`"}`), requestID)
			return
		}
		if verified.Signer != identity.Observer {
			writeSigned(w, r, source, 403, []byte(`{"error":"forbidden"}`), verified.RequestID)
			return
		}
		if verified.Method != r.Method || verified.Path != r.URL.Path || !verified.Current(cfg.Now()) {
			status := 401
			reason := "destination_mismatch"
			if !verified.Current(cfg.Now()) {
				status = 403
				reason = "credential_not_current"
			}
			writeSigned(w, r, source, status, []byte(`{"error":"`+reason+`"}`), verified.RequestID)
			return
		}
		body := []byte(verified.Body)
		if string(verified.Body) == "null" {
			body = []byte{}
		}
		ctx := context.WithValue(r.Context(), signatureKey{}, verifiedSignature{id: verified.Signer, current: verified.Current})
		cloned := r.Clone(ctx)
		cloned.Body = io.NopCloser(bytes.NewReader(body))
		cloned.ContentLength = int64(len(body))
		captured := &bodyCapture{ResponseWriter: w, status: http.StatusOK}
		inner.ServeHTTP(captured, cloned)
		writeSigned(w, r, source, captured.status, captured.body.Bytes(), verified.RequestID)
	})
}

func Server(source identity.Material, cfg Config, audit io.Writer) *http.Server {
	if audit == nil {
		audit = io.Discard
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	server := &http.Server{Handler: observe(SignedHandler(source, cfg), cfg.Role, audit),
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       2 * time.Second, WriteTimeout: 12 * time.Second, MaxHeaderBytes: 8192,
		ErrorLog: log.New(audit, "control_http: ", 0)}
	server.SetKeepAlivesEnabled(false)
	return server
}
