package httpjws

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"poc-spire/internal/kafkacmd"
)

// Client signs each logical request body and accepts a response only when the
// named server SVID signed that same method, path and request ID.
func Client(svid x509svid.Source, bundle x509bundle.Source, server spiffeid.ID) (*http.Client, *http.Transport) {
	base := &http.Transport{DisableKeepAlives: true, ForceAttemptHTTP2: false, ResponseHeaderTimeout: 5 * time.Second, MaxConnsPerHost: 8}
	client := &http.Client{Transport: &roundTripper{base: base, svid: svid, bundle: bundle, server: server}, Timeout: 8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return client, base
}

type roundTripper struct {
	base   http.RoundTripper
	svid   x509svid.Source
	bundle x509bundle.Source
	server spiffeid.ID
}

func (t *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.svid == nil || t.bundle == nil || req.URL.Scheme != "http" {
		return nil, errors.New("signed_http_unavailable")
	}
	logical := []byte{}
	if req.Body != nil {
		var err error
		logical, err = io.ReadAll(io.LimitReader(req.Body, MaxPayload+1))
		req.Body.Close()
		if err != nil || len(logical) > MaxPayload {
			return nil, errors.New("payload_bounds")
		}
	}
	body := json.RawMessage("null")
	if len(logical) > 0 {
		body = logical
	}
	requestID := requestID(req.URL.Path, logical)
	svid, err := t.svid.GetX509SVID()
	if err != nil {
		return nil, errors.New("signing_svid_unavailable")
	}
	compact, err := Sign(svid, req.Method, req.URL.Path, requestID, body, time.Now())
	if err != nil {
		return nil, err
	}
	outgoing := req.Clone(req.Context())
	outgoing.Body = io.NopCloser(strings.NewReader(compact))
	outgoing.ContentLength = int64(len(compact))
	outgoing.Header = req.Header.Clone()
	outgoing.Header.Set("Content-Type", "application/jose")
	response, err := t.base.RoundTrip(outgoing)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, MaxValue+1))
	response.Body.Close()
	if err != nil || len(raw) > MaxValue {
		return nil, errors.New("response_signature_rejected")
	}
	verified, fail := Verify(string(raw), t.bundle, time.Now())
	if fail != nil || verified.Signer != t.server || verified.Method != req.Method || verified.Path != req.URL.Path || verified.RequestID != requestID {
		return nil, errors.New("response_signature_rejected")
	}
	response.Body = io.NopCloser(bytes.NewReader(verified.Body))
	response.ContentLength = int64(len(verified.Body))
	return response, nil
}

func requestID(path string, logical []byte) string {
	if len(logical) > 0 {
		fields, err := kafkacmd.StrictJSON(logical)
		if err == nil {
			if id, ok := fields["request_id"].(string); ok && kafkacmd.RequestID.MatchString(id) {
				return id
			}
		}
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		id := path[i+1:]
		if kafkacmd.RequestID.MatchString(id) && id != "balances" && id != "outcomes" && id != "transfer" && id != "attempt" {
			return id
		}
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "request"
	}
	return "r-" + hex.EncodeToString(b[:])
}
