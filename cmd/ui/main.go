package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"poc-spire/internal/control"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/moneyreader"
	"poc-spire/internal/web"
)

func run() error {
	mode := flag.String("mode", "local", "local Unix-only browser profile or public assumed-operator HTTPS demo")
	origin := flag.String("origin", "http://127.0.0.1:8080", "exact browser origin")
	address := flag.String("listen", "unix:///state/browser.sock", "local Unix socket or remote HTTPS address")
	admin := flag.String("admin", "/state/admin.sock", "owner-only local login socket")
	target := flag.String("money", "https://money:8443", "fixed Money address")
	tlsCert := flag.String("tls-cert", "", "provisioned remote browser certificate file")
	tlsKey := flag.String("tls-key", "", "provisioned remote browser private key file")
	kafkaStream := flag.String("kafka-stream", "", "opt-in combined demo stream (no broker credentials)")
	flag.Parse()
	if *mode != "local" && *mode != "demo" {
		return errors.New("unknown UI profile")
	}
	local := *mode == "local"
	if local && (!strings.HasPrefix(*address, "unix://") || *admin == "" || *tlsCert != "" || *tlsKey != "") {
		return errors.New("local UI requires private Unix sockets; remote configuration is separate")
	}
	if !local && (strings.HasPrefix(*address, "unix://") || *tlsCert == "" || *tlsKey == "") {
		return errors.New("public demo requires browser HTTPS provisioning")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	config := web.Config{Origin: *origin, Local: local, KafkaStream: *kafkaStream}
	var browserTLS *tls.Config
	if !local {
		if _, err := browserCertificate(*tlsCert, *tlsKey, *origin); err != nil {
			return err
		}
		browserTLS = &tls.Config{MinVersion: tls.VersionTLS13, GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return browserCertificate(*tlsCert, *tlsKey, *origin)
		}}
	}
	startup, stop := context.WithTimeout(ctx, 20*time.Second)
	source, err := identity.Source(startup, "unix:///workload/api.sock")
	stop()
	if err != nil {
		return errors.New("UI workload identity unavailable")
	}
	defer source.Close()
	svid, err := source.GetX509SVID()
	if err != nil || svid.ID != identity.Observer {
		return errors.New("unexpected UI workload identity")
	}
	if fixtureRun(ctx, source) {
		return nil
	}
	deps := web.Dependencies{
		Execute: func(ctx context.Context, kind, id string) control.Result {
			socket := "/trusted-control/api.sock"
			if kind == "unidentified" {
				socket = "/untrusted-control/api.sock"
			}
			return control.Call(ctx, socket, id)
		},
		Balances: func(ctx context.Context) (any, error) {
			client, transport := identity.HTTPClient(identity.ClientTLS(source, source, identity.Money))
			defer transport.CloseIdleConnections()
			req, _ := http.NewRequestWithContext(ctx, "GET", *target+"/balances", nil)
			res, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer res.Body.Close()
			data, err := io.ReadAll(io.LimitReader(res.Body, 4097))
			if err != nil || res.StatusCode != 200 || len(data) > 4096 {
				return nil, errors.New("Money read unavailable")
			}
			var value map[string]any
			if json.Unmarshal(data, &value) != nil {
				return nil, errors.New("invalid Money read")
			}
			value["server_verified"] = true
			value["expected_server_id"] = identity.Money.String()
			return value, nil
		}}
	if *kafkaStream != "" {
		observer := moneyreader.Observer{Target: *target, SVID: source, Bundle: source}
		socketFor := func(scenario string) string {
			if scenario == "kafka-unidentified" {
				return "/untrusted-control/api.sock"
			}
			return "/trusted-control/api.sock"
		}
		deps.Kafka = &web.KafkaDependencies{Metadata: observer.Metadata, Receipt: observer.Receipt, Lookup: func(ctx context.Context, scenario, id string) (kafkacaller.Metadata, int, error) {
			return kafkacaller.Call(ctx, socketFor(scenario), nil, id)
		}, Publish: func(ctx context.Context, scenario string, request kafkacaller.PublishRequest) (kafkacaller.Metadata, int, error) {
			return kafkacaller.Call(ctx, socketFor(scenario), &request, request.RequestID)
		}}
	}
	app, err := web.New(ctx, config, deps)
	if err != nil {
		return err
	}
	var listener net.Listener
	if local {
		listener, err = net.Listen("unix", strings.TrimPrefix(*address, "unix://"))
		if err == nil {
			err = os.Chmod(strings.TrimPrefix(*address, "unix://"), 0o600)
		}
	} else {
		listener, err = net.Listen("tcp", *address)
	}
	if err != nil {
		return errors.New("browser listener unavailable")
	}
	defer listener.Close()
	server := &http.Server{Handler: app, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 8 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 8192, TLSConfig: browserTLS}
	errs := make(chan error, 2)
	if local {
		go func() { errs <- app.LocalAdmin(ctx, *admin, 1000) }()
	}
	go func() {
		if local {
			errs <- server.Serve(listener)
		} else {
			errs <- server.ServeTLS(listener, "", "")
		}
	}()
	select {
	case <-ctx.Done():
	case err = <-errs:
		if err != nil && err != http.ErrServerClosed {
			return errors.New("UI listener stopped")
		}
	}
	cancel()
	shutdown, done := context.WithTimeout(context.Background(), 2*time.Second)
	defer done()
	server.Shutdown(shutdown)
	app.Wait()
	return nil
}

// The deployment publishes a single PEM atomically, so certificate renewal
// neither mixes certificate/key generations nor resets sessions or the ledger.
func browserCertificate(certFile, keyFile, origin string) (*tls.Certificate, error) {
	var pair tls.Certificate
	var err error
	if certFile == keyFile {
		var data []byte
		data, err = os.ReadFile(certFile)
		if err == nil {
			pair, err = tls.X509KeyPair(data, data)
		}
	} else {
		pair, err = tls.LoadX509KeyPair(certFile, keyFile)
	}
	if err != nil || len(pair.Certificate) == 0 {
		return nil, errors.New("browser TLS configuration unavailable")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	u, e := url.Parse(origin)
	if err != nil || e != nil || leaf.VerifyHostname(u.Hostname()) != nil || time.Now().Before(leaf.NotBefore) || !time.Now().Before(leaf.NotAfter) {
		return nil, errors.New("browser certificate does not match origin/validity")
	}
	return &pair, nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
