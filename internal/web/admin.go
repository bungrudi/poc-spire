package web

import (
	"context"
	"net"
	"net/http"
	"os"
	"time"

	"poc-spire/internal/control"
)

type adminUID struct{}

// LocalAdmin has no TCP listener. Kernel peer credentials bind login-code
// issuance to the existing OS owner; codes/sessions remain solely in memory.
func (a *App) LocalAdmin(ctx context.Context, path string, uid uint32) error {
	if !a.config.Local {
		return os.ErrPermission
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	defer l.Close()
	if err = os.Chmod(path, 0o600); err != nil {
		return err
	}
	s := &http.Server{ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxHeaderBytes: 2048,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			p, err := control.PeerUID(c)
			if err != nil {
				return ctx
			}
			return context.WithValue(ctx, adminUID{}, p)
		},
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, ok := r.Context().Value(adminUID{}).(uint32)
			if !ok || peer != uid {
				http.Error(w, "owner denied", 403)
				return
			}
			if r.Method != "POST" || r.URL.Path != "/login-code" || r.ContentLength != 0 {
				http.NotFound(w, r)
				return
			}
			value, err := a.issueLocal(Principal{Issuer: "urn:poc-spire:local", Subject: "uid:1000", Role: "operator"})
			if err != nil {
				http.Error(w, "login capacity reached", 503)
				return
			}
			headers(w)
			reply(w, 200, map[string]string{"code": value})
		})}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		s.Shutdown(c)
	}()
	err = s.Serve(l)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
