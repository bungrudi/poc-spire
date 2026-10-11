package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"time"
)

type Principal struct{ Issuer, Subject, Role string }

func (p Principal) key() string { return p.Issuer + "\x00" + p.Subject }
func random() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("secure randomness unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
func equal(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

type session struct {
	Principal     Principal
	CSRF          string
	Created, Seen time.Time
}
type code struct {
	Principal Principal
	Expires   time.Time
}

func (a *App) cookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: !a.config.Local, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}
func (a *App) login(w http.ResponseWriter, r *http.Request, p Principal) {
	if p.Issuer == "" || p.Subject == "" || (p.Role != "operator" && p.Role != "viewer") {
		http.Error(w, "login denied", 403)
		return
	}
	if old, err := r.Cookie("poc_session"); err == nil {
		delete(a.sessions, old.Value)
	}
	if len(a.sessions) >= 128 {
		http.Error(w, "session capacity reached", 503)
		return
	}
	id := random()
	now := a.now()
	a.sessions[id] = session{Principal: p, CSRF: random(), Created: now, Seen: now}
	a.cookie(w, "poc_session", id, 3600)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (a *App) authenticate(r *http.Request) (session, bool) {
	c, err := r.Cookie("poc_session")
	if err != nil {
		return session{}, false
	}
	s, ok := a.sessions[c.Value]
	if !ok {
		return session{}, false
	}
	now := a.now()
	if now.Sub(s.Created) >= time.Hour || now.Sub(s.Seen) >= 15*time.Minute {
		delete(a.sessions, c.Value)
		return session{}, false
	}
	s.Seen = now
	a.sessions[c.Value] = s
	return s, true
}

// A demo session records a declared assumption, never a verified human identity.
// It exists only to bind browser CSRF tokens and bounded session lifetime.
func (a *App) assumeOperator(w http.ResponseWriter) (session, bool) {
	a.pruneAuth()
	if len(a.sessions) >= 128 {
		return session{}, false
	}
	now := a.now()
	s := session{Principal: Principal{Issuer: "urn:poc-spire:assumed", Subject: "demo-operator", Role: "operator"}, CSRF: random(), Created: now, Seen: now}
	id := random()
	a.sessions[id] = s
	a.cookie(w, "poc_session", id, 3600)
	return s, true
}
func (a *App) issueLocal(p Principal) (string, error) {
	if !a.config.Local {
		return "", errors.New("local login disabled")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pruneAuth()
	if len(a.codes) >= 16 {
		return "", errors.New("login code capacity reached")
	}
	value := random()
	sum := sha256.Sum256([]byte(value))
	key := base64.RawURLEncoding.EncodeToString(sum[:])
	a.codes[key] = code{Principal: p, Expires: a.now().Add(time.Minute)}
	return value, nil
}
func (a *App) pruneAuth() {
	now := a.now()
	for k, c := range a.codes {
		if !now.Before(c.Expires) {
			delete(a.codes, k)
		}
	}
	for k, s := range a.sessions {
		if now.Sub(s.Created) >= time.Hour || now.Sub(s.Seen) >= 15*time.Minute {
			delete(a.sessions, k)
		}
	}
}
func (a *App) authRoute(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/auth/local":
		if !a.config.Local || r.Method != "POST" {
			http.NotFound(w, r)
			return true
		}
		if !a.originOK(r) {
			http.Error(w, "origin denied", 403)
			return true
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		if r.ParseForm() != nil {
			http.Error(w, "invalid login", 400)
			return true
		}
		value := r.Form.Get("code")
		sum := sha256.Sum256([]byte(value))
		key := base64.RawURLEncoding.EncodeToString(sum[:])
		c, ok := a.codes[key]
		delete(a.codes, key)
		if !ok || !a.now().Before(c.Expires) {
			http.Error(w, "login denied", 401)
			return true
		}
		a.login(w, r, c.Principal)
		return true
	}
	return false
}
