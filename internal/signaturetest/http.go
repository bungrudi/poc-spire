// Package signaturetest is bounded HTTP instrumentation for security fixtures.
// Application entrypoints use httpjws.Client and the Money/control handlers.
package signaturetest

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"io"
	"net/http"
	"net/http/httptest"
	"poc-spire/internal/httpjws"
	"strings"
	"time"
)

type Response struct {
	*httpjws.Verified
	Authority string
}

type Material struct {
	SVID   *x509svid.SVID
	Bundle x509bundle.Source
}

func (m Material) GetX509SVID() (*x509svid.SVID, error) {
	if m.SVID == nil {
		return nil, errors.New("no_svid")
	}
	return m.SVID, nil
}
func (m Material) GetX509BundleForTrustDomain(td spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return m.Bundle.GetX509BundleForTrustDomain(td)
}

// Exchange allows explicit unsigned/frozen fixtures; every response is verified.
func Exchange(ctx context.Context, svid *x509svid.SVID, bundle x509bundle.Source, server spiffeid.ID, target, method, path, id string, body []byte, at time.Time) (*Response, int, error) {
	if len(body) == 0 {
		body = []byte("null")
	}
	wire := string(body)
	var err error
	if svid != nil {
		wire, err = httpjws.Sign(svid, method, path, id, body, at)
		if err != nil {
			return nil, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, target+path, strings.NewReader(wire))
	if err != nil {
		return nil, 0, err
	}
	if req.URL.Scheme != "http" {
		return nil, 0, errors.New("fixture_requires_http")
	}
	req.Header.Set("X-Spiffe-ID", "spiffe://poc.internal/caller/trusted")
	req.Header.Set("X-Forwarded-Spiffe-ID", "spiffe://poc.internal/caller/trusted")
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, httpjws.MaxValue+1))
	if err != nil || len(raw) > httpjws.MaxValue {
		return nil, 0, errors.New("response_bounds")
	}
	verified, fail := httpjws.Verify(string(raw), bundle, time.Now())
	if fail != nil {
		return nil, 0, errors.New("response_signature_rejected")
	}
	if verified.Signer != server {
		return nil, 0, errors.New("server_identity_mismatch")
	}
	if verified.Method != method || verified.Path != path || (verified.RequestID != id && !(verified.RequestID == "unsigned" && response.StatusCode == 401)) {
		return nil, 0, errors.New("response_correlation_rejected")
	}
	var header struct {
		X5C []string `json:"x5c"`
	}
	decoded, _ := base64.RawURLEncoding.DecodeString(strings.Split(string(raw), ".")[0])
	json.Unmarshal(decoded, &header)
	authority := ""
	if len(header.X5C) > 0 {
		der, _ := base64.StdEncoding.DecodeString(header.X5C[0])
		cert, err := x509.ParseCertificate(der)
		if err == nil {
			authority = hex.EncodeToString(cert.AuthorityKeyId)
		}
	}
	return &Response{verified, authority}, response.StatusCode, nil
}

type peerKey struct{}

func Peer(r *http.Request) spiffeid.ID {
	id, _ := r.Context().Value(peerKey{}).(spiffeid.ID)
	return id
}

// Handler wraps test responses with genuine SVID signatures; no TLS is used.
func Handler(svid *x509svid.SVID, bundle x509bundle.Source, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, httpjws.MaxValue+1))
		v, fail := httpjws.Verify(string(raw), bundle, time.Now())
		id := "unsigned"
		status := 401
		body := []byte(`{"error":"invalid_signature"}`)
		if fail == nil && v.Method == r.Method && v.Path == r.URL.Path {
			id = v.RequestID
			clone := r.Clone(context.WithValue(r.Context(), peerKey{}, v.Signer))
			logical := []byte(v.Body)
			if string(logical) == "null" {
				logical = nil
			}
			clone.Body = io.NopCloser(bytes.NewReader(logical))
			captured := httptest.NewRecorder()
			next.ServeHTTP(captured, clone)
			status = captured.Code
			body = captured.Body.Bytes()
		}
		compact, err := httpjws.Sign(svid, r.Method, r.URL.Path, id, body, time.Now())
		if err != nil {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/jose")
		w.WriteHeader(status)
		io.WriteString(w, compact)
	})
}
