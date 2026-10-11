package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"poc-spire/internal/control"
)

func testApp(t *testing.T, config Config, execute func(context.Context, string, string) control.Result) *App {
	t.Helper()
	config.Local = true
	config.Origin = "http://127.0.0.1:8080"
	a, err := New(context.Background(), config, Dependencies{Execute: execute, Balances: func(context.Context) (any, error) { return map[string]int{"version": 0}, nil }})
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func request(a *App, method, path string, body []byte, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, bytes.NewReader(body))
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if method == "POST" {
		r.Header.Set("Origin", a.config.Origin)
		r.Header.Set("X-CSRF-Token", csrf)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func signin(t *testing.T, a *App, p Principal) (*http.Cookie, string) {
	t.Helper()
	code, err := a.issueLocal(p)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", a.config.Origin+"/auth/local", strings.NewReader(url.Values{"code": {code}}.Encode()))
	r.Header.Set("Origin", a.config.Origin)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 303 {
		t.Fatalf("login %d", w.Code)
	}
	c := w.Result().Cookies()[0]
	if !c.HttpOnly || c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatal("local cookie scope")
	}
	var session map[string]string
	json.Unmarshal(request(a, "GET", "/api/session", nil, c, "").Body.Bytes(), &session)
	return c, session["csrf_token"]
}
func attemptBody(key, caller string) []byte {
	return []byte(fmt.Sprintf(`{"browser_key":%q,"caller":%q}`, key, caller))
}

var operator = Principal{Issuer: "urn:poc-spire:local", Subject: "uid:1000", Role: "operator"}

func TestWebAuthorizationBeforeDispatch(t *testing.T) {
	var calls atomic.Int64
	a := testApp(t, Config{}, func(_ context.Context, _, id string) control.Result {
		calls.Add(1)
		return control.Result{RequestID: id, Outcome: "transfer_succeeded"}
	})
	op, csrf := signin(t, a, operator)
	viewer, _ := signin(t, a, Principal{Issuer: operator.Issuer, Subject: "viewer", Role: "viewer"})
	for _, tc := range []struct {
		name, method, path string
		c                  *http.Cookie
		csrf               string
		modify             func(*http.Request)
		status             int
	}{
		{"anonymous", "POST", "/api/attempts", nil, "", nil, 401},
		{"spoofed-identity", "POST", "/api/attempts", nil, "", func(r *http.Request) {
			r.Header.Set("X-User", "uid:1000")
			r.Header.Set("X-Role", "operator")
			r.Header.Set("X-SPIFFE-ID", "spiffe://poc.internal/caller/trusted")
		}, 401},
		{"viewer", "POST", "/api/attempts", viewer, "", nil, 403},
		{"missing-csrf", "POST", "/api/attempts", op, "", nil, 403},
		{"wrong-csrf", "POST", "/api/attempts", op, "wrong", nil, 403},
		{"foreign-origin", "POST", "/api/attempts", op, csrf, func(r *http.Request) { r.Header.Set("Origin", "https://foreign.example") }, 403},
		{"foreign-host", "POST", "/api/attempts", op, csrf, func(r *http.Request) { r.Host = "foreign.example"; r.Header.Set("X-Forwarded-Host", a.host) }, 403},
		{"cross-site-fetch", "POST", "/api/attempts", op, csrf, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }, 403},
		{"state-changing-get", "GET", "/api/attempts", op, csrf, nil, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, a.config.Origin+tc.path, bytes.NewReader(attemptBody(strings.Repeat("a", 32), "trusted")))
			if tc.c != nil {
				r.AddCookie(tc.c)
			}
			r.Header.Set("Origin", a.config.Origin)
			r.Header.Set("X-CSRF-Token", tc.csrf)
			if tc.modify != nil {
				tc.modify(r)
			}
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d want %d", w.Code, tc.status)
			}
		})
	}
	if calls.Load() != 0 || len(a.attempts) != 0 {
		t.Fatal("unauthorized request dispatched/populated history")
	}
	if request(a, "GET", "/api/balances", nil, viewer, "").Code != 200 {
		t.Fatal("viewer cannot read")
	}
	if request(a, "POST", "/api/attempts", []byte(`{"browser_key":"too-short","caller":"trusted"}`), op, csrf).Code != 400 {
		t.Fatal("short key accepted")
	}
	if request(a, "POST", "/api/attempts", []byte(`{"browser_key":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","caller":"trusted","url":"http://evil"}`), op, csrf).Code != 400 {
		t.Fatal("arbitrary target accepted")
	}
	if request(a, "POST", "/api/attempts", bytes.Repeat([]byte("x"), 1025), op, csrf).Code != 400 {
		t.Fatal("oversized body accepted")
	}
	if len(a.attempts) != 0 {
		t.Fatal("bad payload consumed capacity")
	}
}

func TestInterruptedConcurrentRepeatedAttemptsAndOwnership(t *testing.T) {
	start := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	a := testApp(t, Config{Concurrent: 1}, func(_ context.Context, _, id string) control.Result {
		if calls.Add(1) == 1 {
			close(start)
		}
		<-release
		return control.Result{RequestID: id, Outcome: "transfer_succeeded", ServerVerified: true}
	})
	c, csrf := signin(t, a, operator)
	key := strings.Repeat("k", 32)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("POST", a.config.Origin+"/api/attempts", bytes.NewReader(attemptBody(key, "trusted"))).WithContext(ctx)
	r.AddCookie(c)
	r.Header.Set("Origin", a.config.Origin)
	r.Header.Set("X-CSRF-Token", csrf)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	<-start
	if request(a, "POST", "/api/attempts", attemptBody(strings.Repeat("b", 32), "trusted"), c, csrf).Code != 429 {
		t.Fatal("concurrency limit")
	}
	for i := 0; i < 20; i++ {
		if request(a, "POST", "/api/attempts", attemptBody(key, "trusted"), c, csrf).Code != 202 {
			t.Fatal("retry not accepted")
		}
	}
	if request(a, "POST", "/api/attempts", attemptBody(key, "unidentified"), c, csrf).Code != 409 {
		t.Fatal("caller conflict")
	}
	other, _ := signin(t, a, Principal{Issuer: operator.Issuer, Subject: "other", Role: "viewer"})
	id := RequestID(operator, key)
	if request(a, "GET", "/api/attempts/"+id, nil, other, "").Code != 404 {
		t.Fatal("cross-owner result disclosure")
	}
	close(release)
	a.Wait()
	if calls.Load() != 1 {
		t.Fatal("repeated key dispatched more than once")
	}
	if request(a, "POST", "/api/attempts", attemptBody(key, "trusted"), c, csrf).Code != 202 || calls.Load() != 1 {
		t.Fatal("completed retry redispatched")
	}
	if RequestID(operator, key) == RequestID(Principal{Issuer: operator.Issuer, Subject: "different"}, key) {
		t.Fatal("principal not bound")
	}
}

func TestLimitsNoEvictionAndRetryableResult(t *testing.T) {
	var calls atomic.Int64
	a := testApp(t, Config{Capacity: 2, Rate: 2}, func(_ context.Context, _, id string) control.Result {
		n := calls.Add(1)
		out := "transfer_succeeded"
		if n == 1 {
			out = "unknown_commit"
		}
		return control.Result{RequestID: id, Outcome: out}
	})
	c, csrf := signin(t, a, operator)
	key := strings.Repeat("k", 32)
	request(a, "POST", "/api/attempts", attemptBody(key, "trusted"), c, csrf)
	a.Wait()
	request(a, "POST", "/api/attempts", attemptBody(key, "trusted"), c, csrf)
	a.Wait()
	if calls.Load() != 2 {
		t.Fatal("unknown result not retried")
	}
	if request(a, "POST", "/api/attempts", attemptBody(strings.Repeat("b", 32), "trusted"), c, csrf).Code != 429 {
		t.Fatal("rate limit")
	}
	a.mu.Lock()
	a.now = func() time.Time { return time.Now().Add(61 * time.Second) }
	a.mu.Unlock()
	request(a, "POST", "/api/attempts", attemptBody(strings.Repeat("b", 32), "trusted"), c, csrf)
	a.Wait()
	if request(a, "POST", "/api/attempts", attemptBody(strings.Repeat("c", 32), "trusted"), c, csrf).Code != 503 {
		t.Fatal("capacity limit")
	}
	if request(a, "GET", "/api/attempts/"+RequestID(operator, key), nil, c, "").Code != 200 {
		t.Fatal("old key evicted")
	}
}

func TestSessionExpiryRotationLogoutAndOneUseCode(t *testing.T) {
	a := testApp(t, Config{}, func(context.Context, string, string) control.Result { return control.Result{} })
	c, csrf := signin(t, a, operator)
	r := httptest.NewRequest("POST", a.config.Origin+"/auth/local", strings.NewReader("code=wrong"))
	r.Header.Set("Origin", a.config.Origin)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("invalid code accepted")
	}
	value, _ := a.issueLocal(operator)
	login := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", a.config.Origin+"/auth/local", strings.NewReader(url.Values{"code": {value}}.Encode()))
		r.AddCookie(c)
		r.Header.Set("Origin", a.config.Origin)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	next := login()
	if next.Code != 303 || login().Code != 401 {
		t.Fatal("code not single use")
	}
	if request(a, "GET", "/api/session", nil, c, "").Code != 401 {
		t.Fatal("old session not rotated")
	}
	c = next.Result().Cookies()[0]
	var s map[string]string
	json.Unmarshal(request(a, "GET", "/api/session", nil, c, "").Body.Bytes(), &s)
	csrf = s["csrf_token"]
	if request(a, "POST", "/auth/logout", nil, c, "").Code != 403 {
		t.Fatal("logout CSRF")
	}
	if request(a, "POST", "/auth/logout", nil, c, csrf).Code != 200 || request(a, "GET", "/api/session", nil, c, "").Code != 401 {
		t.Fatal("logout session invalidation")
	}
	c, _ = signin(t, a, operator)
	a.mu.Lock()
	a.now = func() time.Time { return time.Now().Add(16 * time.Minute) }
	a.mu.Unlock()
	if request(a, "GET", "/api/session", nil, c, "").Code != 401 {
		t.Fatal("idle expiry")
	}
	if _, err := New(context.Background(), Config{Origin: "http://0.0.0.0:8080", Local: true}, a.deps); err == nil {
		t.Fatal("non-loopback local origin")
	}
	if _, err := New(context.Background(), Config{Origin: "http://example.test", Local: false}, a.deps); err == nil {
		t.Fatal("public demo accepted plaintext origin")
	}
}

func TestAssumedDemoSessionRequiresExplicitHTTPSAndCSRF(t *testing.T) {
	var calls atomic.Int64
	a, err := New(context.Background(), Config{Origin: "https://spire.adianto.dev"}, Dependencies{
		Execute: func(_ context.Context, _, id string) control.Result {
			calls.Add(1)
			return control.Result{RequestID: id, Outcome: "transfer_succeeded"}
		}, Balances: func(context.Context) (any, error) { return nil, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", "https://spire.adianto.dev/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "operator identity is assumed, not authenticated") {
		t.Fatal("demo must disclose that human identity is an assumption")
	}
	c := w.Result().Cookies()[0]
	if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe demo cookie")
	}
	s := a.sessions[c.Value]
	if s.Principal != (Principal{Issuer: "urn:poc-spire:assumed", Subject: "demo-operator", Role: "operator"}) {
		t.Fatal("unexpected assumed principal")
	}
	for _, csrf := range []string{"", "forged"} {
		r := httptest.NewRequest("POST", a.config.Origin+"/api/attempts", bytes.NewReader(attemptBody(strings.Repeat("k", 32), "trusted")))
		r.AddCookie(c)
		r.Header.Set("Origin", a.config.Origin)
		r.Header.Set("X-CSRF-Token", csrf)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("assumed operator bypassed CSRF")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("CSRF rejection dispatched a transfer")
	}
	for _, path := range []string{"/auth/login", "/auth/callback", "/auth/local"} {
		r := httptest.NewRequest("GET", a.config.Origin+path, nil)
		r.AddCookie(c)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatal("public demo exposed a login endpoint")
		}
	}
}
