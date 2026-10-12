// Package control exposes bounded caller commands only on private Unix sockets.
package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type Request struct {
	RequestID string `json:"request_id"`
}
type Result struct {
	RequestID           string          `json:"request_id"`
	Stage               string          `json:"stage"`
	Outcome             string          `json:"outcome"`
	IdentityEligibility string          `json:"identity_eligibility,omitempty"`
	MoneyStatus         int             `json:"money_http_status,omitempty"`
	ServerVerified      bool            `json:"server_verified"`
	MoneyResult         json.RawMessage `json:"money_result,omitempty"`
}
type Executor func(context.Context, string) Result
type uidKey struct{}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func PeerUID(conn net.Conn) (uint32, error) {
	u, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, errors.New("not Unix connection")
	}
	raw, err := u.SyscallConn()
	if err != nil {
		return 0, err
	}
	var credentials *unix.Ucred
	var credentialError error
	err = raw.Control(func(fd uintptr) {
		credentials, credentialError = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if err != nil || credentialError != nil || credentials == nil {
		return 0, errors.New("peer credentials unavailable")
	}
	return credentials.Uid, nil
}

func endpoint(operatorUID uint32, execute Executor, kafka ...http.Handler) http.Handler {
	limit := make(chan struct{}, 2)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		uid, ok := r.Context().Value(uidKey{}).(uint32)
		if !ok || uid != operatorUID {
			http.Error(w, "control peer denied", 403)
			return
		}
		if len(kafka) > 0 && kafka[0] != nil && (r.URL.Path == "/attempt/kafka" || strings.HasPrefix(r.URL.Path, "/attempt/kafka/")) {
			select {
			case limit <- struct{}{}:
				defer func() { <-limit }()
			default:
				http.Error(w, "caller busy", 429)
				return
			}
			timeout := 10 * time.Second
			if r.Method == "GET" {
				timeout = 2 * time.Second
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			kafka[0].ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if r.Method != "POST" || r.URL.Path != "/attempt" {
			http.Error(w, "unsupported command", 404)
			return
		}
		select {
		case limit <- struct{}{}:
			defer func() { <-limit }()
		default:
			http.Error(w, "caller busy", 429)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var request Request
		if decoder.Decode(&request) != nil || !idPattern.MatchString(request.RequestID) {
			http.Error(w, "invalid command", 400)
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			http.Error(w, "invalid command", 400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		json.NewEncoder(w).Encode(execute(ctx, request.RequestID))
	})
}

func Serve(ctx context.Context, path string, operatorUID uint32, execute Executor, kafka ...http.Handler) error {
	listener, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	defer listener.Close()
	if err = os.Chmod(path, 0o660); err != nil {
		return err
	}
	server := &http.Server{Handler: endpoint(operatorUID, execute, kafka...), ReadHeaderTimeout: time.Second,
		ReadTimeout: 2 * time.Second, WriteTimeout: 12 * time.Second, MaxHeaderBytes: 2048,
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			uid, err := PeerUID(conn)
			if err != nil {
				return ctx
			}
			return context.WithValue(ctx, uidKey{}, uid)
		}}
	server.SetKeepAlivesEnabled(false)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
