// backend-money is a test fixture using the real Money handler and SDK TLS.
// It drops one selected successful response after commit; it adds no auth bypass.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"time"

	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
	"poc-spire/internal/money"
)

func run() int {
	address := flag.String("listen", "127.0.0.1:8443", "private test listener")
	dropID := flag.String("drop-response-for", "lost-response", "one fixed test request ID")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	source, err := identity.Source(ctx, "unix:///workload/api.sock")
	if err != nil {
		return 1
	}
	defer source.Close()
	s, err := source.GetX509SVID()
	if err != nil || s.ID != identity.Money {
		return 1
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		return 1
	}
	defer listener.Close()
	server := money.Server(source, ledger.New(10000), os.Stdout)
	base := server.Handler
	var dropped atomic.Bool
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/transfer" && !dropped.Load() {
			body, err := io.ReadAll(io.LimitReader(r.Body, 4097))
			if err != nil {
				return
			}
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
			var input ledger.Request
			if len(body) <= 4096 && json.Unmarshal(body, &input) == nil && input.RequestID == *dropID {
				captured := httptest.NewRecorder()
				base.ServeHTTP(captured, r)
				if captured.Code == 200 && dropped.CompareAndSwap(false, true) {
					connection, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						connection.Close()
					}
					return
				}
				for name, values := range captured.Header() {
					for _, value := range values {
						w.Header().Add(name, value)
					}
				}
				w.WriteHeader(captured.Code)
				w.Write(captured.Body.Bytes())
				return
			}
		}
		base.ServeHTTP(w, r)
	})
	if err := server.ServeTLS(listener, "", ""); err != nil && err != http.ErrServerClosed {
		return 1
	}
	return 0
}
func main() { os.Exit(run()) }
