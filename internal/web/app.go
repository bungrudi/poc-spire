// Package web implements bounded browser authorization and asynchronous caller
// orchestration. The local profile is Unix-only behind a loopback bridge.
package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"poc-spire/internal/control"
	"poc-spire/internal/kafkacmd"
)

type Config struct {
	KafkaStream                string
	Run                        string
	Origin                     string
	Local                      bool
	Capacity, Concurrent, Rate int
}
type Dependencies struct {
	RunValid func() bool
	Kafka    *KafkaDependencies
	Execute  func(context.Context, string, string) control.Result
	Balances func(context.Context) (any, error)
}
type Attempt struct {
	Scenario string          `json:"scenario,omitempty"`
	StreamID string          `json:"stream_id,omitempty"`
	Kafka    *KafkaResult    `json:"kafka_evidence,omitempty"`
	ID       string          `json:"attempt_id"`
	Caller   string          `json:"caller"`
	Status   string          `json:"status"`
	Created  time.Time       `json:"created_at"`
	Updated  time.Time       `json:"updated_at"`
	Result   *control.Result `json:"caller_evidence,omitempty"`
	owner    string
}
type App struct {
	config   Config
	host     string
	deps     Dependencies
	ctx      context.Context
	mu       sync.Mutex
	now      func() time.Time
	sessions map[string]session
	codes    map[string]code
	attempts map[string]*Attempt
	rates    map[string][]time.Time
	active   int
	workers  sync.WaitGroup
}

func validate(config Config) (string, error) {
	u, err := url.Parse(config.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("exact origin required")
	}
	if config.Local {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || ip == nil || !ip.IsLoopback() {
			return "", errors.New("local profile requires literal loopback HTTP origin")
		}
	} else if u.Scheme != "https" {
		return "", errors.New("remote profile requires HTTPS origin")
	}
	return u.Host, nil
}
func New(ctx context.Context, config Config, deps Dependencies) (*App, error) {
	host, err := validate(config)
	if err != nil {
		return nil, err
	}
	if config.Run != "" && (config.Local || !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(config.Run) || deps.RunValid == nil) {
		return nil, errors.New("guarded Kubernetes browser run required")
	}
	if deps.Execute == nil || deps.Balances == nil {
		return nil, errors.New("authenticated workload dependencies required")
	}
	if config.KafkaStream != "" {
		if !kafkacmd.Hex32.MatchString(config.KafkaStream) || deps.Kafka == nil || deps.Kafka.Metadata == nil || deps.Kafka.Receipt == nil || deps.Kafka.Lookup == nil || deps.Kafka.Publish == nil {
			return nil, errors.New("authenticated Kafka outcome dependencies required")
		}
	} else if deps.Kafka != nil {
		return nil, errors.New("Kafka stream required")
	}
	if config.Capacity == 0 {
		config.Capacity = 64
	}
	if config.Concurrent == 0 {
		config.Concurrent = 2
	}
	if config.Rate == 0 {
		config.Rate = 8
	}
	if config.Capacity < 1 || config.Capacity > 1024 || config.Concurrent < 1 || config.Concurrent > 8 || config.Rate < 1 || config.Rate > 60 {
		return nil, errors.New("invalid limits")
	}
	a := &App{config: config, host: host, deps: deps, ctx: ctx, now: time.Now, sessions: map[string]session{}, codes: map[string]code{}, attempts: map[string]*Attempt{}, rates: map[string][]time.Time{}}
	return a, nil
}
func (a *App) Wait() { a.workers.Wait() }
func (a *App) originOK(r *http.Request) bool {
	return r.Header.Get("Origin") == a.config.Origin && r.Header.Get("Sec-Fetch-Site") != "cross-site"
}
func headers(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
}
func reply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	headers(w)
	if r.Host != a.host {
		http.Error(w, "host denied", 403)
		return
	}
	if r.URL.RawQuery != "" && r.URL.Path != "/auth/callback" {
		http.Error(w, "unsupported query", 400)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/healthz" {
		reply(w, 200, map[string]string{"status": "alive"})
		return
	}
	if r.Method == "GET" && r.URL.Path == "/assets/app.js" {
		w.Header().Set("Content-Type", "text/javascript")
		io.WriteString(w, script)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/assets/style.css" {
		w.Header().Set("Content-Type", "text/css")
		io.WriteString(w, style)
		return
	}
	a.mu.Lock()
	handled := a.authRoute(w, r)
	a.mu.Unlock()
	if handled {
		return
	}
	a.mu.Lock()
	s, ok := a.authenticate(r)
	if !ok && !a.config.Local && r.Method == "GET" && r.URL.Path == "/" {
		s, ok = a.assumeOperator(w)
		if !ok {
			a.mu.Unlock()
			http.Error(w, "session capacity reached", 503)
			return
		}
	}
	a.mu.Unlock()
	if !ok {
		if r.Method == "GET" && r.URL.Path == "/" {
			page.Execute(w, map[string]any{"Local": a.config.Local})
			return
		}
		http.Error(w, "authentication required", 401)
		return
	}
	if s.Principal.Role != "operator" && s.Principal.Role != "viewer" {
		http.Error(w, "role denied", 403)
		return
	}
	if r.Method == "POST" {
		if !a.originOK(r) || !equal(r.Header.Get("X-CSRF-Token"), s.CSRF) {
			http.Error(w, "CSRF/origin denied", 403)
			return
		}
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/":
		a.render(w, r, s)
		return
	case r.Method == "GET" && r.URL.Path == "/api/session":
		session := map[string]string{"role": s.Principal.Role, "csrf_token": s.CSRF, "auth_profile": map[bool]string{true: "local OS-owner login", false: "assumed demo operator; human not authenticated"}[a.config.Local]}
		if a.config.Run != "" {
			session["run_id"] = a.config.Run
		}
		reply(w, 200, session)
		return
	case r.Method == "POST" && r.URL.Path == "/auth/logout":
		if c, err := r.Cookie("poc_session"); err == nil {
			a.mu.Lock()
			delete(a.sessions, c.Value)
			a.mu.Unlock()
		}
		a.cookie(w, "poc_session", "", -1)
		reply(w, 200, map[string]string{"status": "logged_out"})
		return
	case r.Method == "GET" && r.URL.Path == "/api/balances":
		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()
		value, err := a.deps.Balances(ctx)
		if err != nil {
			reply(w, 503, map[string]string{"outcome": "infrastructure_failure"})
			return
		}
		reply(w, 200, value)
		return
	case r.Method == "POST" && r.URL.Path == "/api/kafka-attempts":
		if s.Principal.Role != "operator" {
			http.Error(w, "operator required", 403)
			return
		}
		a.createKafka(w, r, s.Principal)
		return
	case r.Method == "POST" && r.URL.Path == "/api/attempts":
		if s.Principal.Role != "operator" {
			http.Error(w, "operator required", 403)
			return
		}
		a.create(w, r, s.Principal)
		return
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/attempts/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/attempts/")
		a.mu.Lock()
		attempt, ok := a.attempts[id]
		if !ok || attempt.owner != s.Principal.key() {
			a.mu.Unlock()
			http.NotFound(w, r)
			return
		}
		copy := *attempt
		a.mu.Unlock()
		reply(w, 200, copy)
		return
	default:
		http.NotFound(w, r)
	}
}

var browserKey = regexp.MustCompile(`^[A-Za-z0-9_-]{32,128}$`)

func RequestID(p Principal, key string) string {
	encoded, _ := json.Marshal([]string{"poc-spire-ui-request-v1", p.Issuer, p.Subject, key})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
func (a *App) create(w http.ResponseWriter, r *http.Request, p Principal) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var in struct {
		Caller string `json:"caller"`
		Key    string `json:"browser_key"`
	}
	var extra any
	if d.Decode(&in) != nil || d.Decode(&extra) != io.EOF || (in.Caller != "trusted" && in.Caller != "unidentified") || !browserKey.MatchString(in.Key) {
		http.Error(w, "invalid fixed attempt", 400)
		return
	}
	a.dispatch(w, r, p, RequestID(p, in.Key), in.Caller, "", "")
}

func (a *App) dispatch(w http.ResponseWriter, r *http.Request, p Principal, id, caller, scenario, stream string) {
	if a.deps.RunValid != nil && !a.deps.RunValid() {
		reply(w, 503, map[string]string{"error": "run_invalid"})
		return
	}
	now := a.now()
	a.mu.Lock()
	old, exists := a.attempts[id]
	if exists {
		if old.Caller != caller || old.Scenario != scenario || old.StreamID != stream || old.owner != p.key() {
			a.mu.Unlock()
			http.Error(w, "key conflicts with caller", 409)
			return
		}
		if old.Status != "retryable" {
			copy := *old
			a.mu.Unlock()
			reply(w, 202, copy)
			return
		}
	} else if len(a.attempts) >= a.config.Capacity {
		a.mu.Unlock()
		http.Error(w, "attempt capacity reached", 503)
		return
	}
	if a.active >= a.config.Concurrent {
		a.mu.Unlock()
		http.Error(w, "attempt concurrency reached", 429)
		return
	}
	hits := a.rates[p.key()]
	valid := hits[:0]
	for _, t := range hits {
		if now.Sub(t) < time.Minute {
			valid = append(valid, t)
		}
	}
	if len(valid) >= a.config.Rate {
		a.rates[p.key()] = valid
		a.mu.Unlock()
		http.Error(w, "attempt rate reached", 429)
		return
	}
	a.rates[p.key()] = append(valid, now)
	if !exists {
		old = &Attempt{ID: id, Caller: caller, Scenario: scenario, StreamID: stream, Created: now, owner: p.key()}
		a.attempts[id] = old
	}
	old.Status = "pending"
	old.Updated = now
	old.Result = nil
	old.Kafka = nil
	a.active++
	copy := *old
	a.workers.Add(1)
	a.mu.Unlock()
	go func() {
		defer a.workers.Done()
		if scenario != "" {
			a.kafkaWorker(old)
			return
		}
		ctx, cancel := context.WithTimeout(a.ctx, 12*time.Second)
		defer cancel()
		result := control.Result{RequestID: id, Stage: "run_guard", Outcome: "unknown_commit"}
		if a.deps.RunValid == nil || a.deps.RunValid() {
			result = a.deps.Execute(ctx, caller, id)
		}
		if result.RequestID != id {
			result = control.Result{RequestID: id, Stage: "caller_control", Outcome: "unknown_commit"}
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		old.Result = &result
		old.Updated = a.now()
		old.Status = "done"
		if result.Outcome == "unknown_commit" || result.Outcome == "infrastructure_failure" {
			old.Status = "retryable"
		}
		a.active--
	}()
	reply(w, 202, copy)
}
