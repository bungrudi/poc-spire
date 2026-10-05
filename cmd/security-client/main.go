// security-client is test instrumentation, not an application trigger endpoint.
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

type report struct {
	Outcome        string          `json:"outcome"`
	Status         int             `json:"money_http_status,omitempty"`
	ServerVerified bool            `json:"server_verified"`
	Body           json.RawMessage `json:"body,omitempty"`
	Checks         map[string]bool `json:"checks,omitempty"`
}

func fakeCertificate() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	u, _ := url.Parse(identity.Trusted.String())
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		URIs: []*url.URL{u}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, err
}
func traceServer(config *tls.Config, verified, mismatch *bool) {
	verify := config.VerifyPeerCertificate
	config.VerifyPeerCertificate = func(raw [][]byte, chains [][]*x509.Certificate) error {
		err := verify(raw, chains)
		if err == nil {
			*verified = true
		} else if strings.HasPrefix(err.Error(), "unexpected ID ") {
			*mismatch = true
		}
		return err
	}
}
func call(config *tls.Config, target, action, id string, amount int64) (report, error) {
	result := report{Outcome: "transport_failure"}
	mismatch := false
	traceServer(config, &result.ServerVerified, &mismatch)
	client, transport := identity.HTTPClient(config)
	defer transport.CloseIdleConnections()
	method, path, body := "GET", "/balances", []byte(nil)
	if action == "transfer" {
		method, path = "POST", "/transfer"
		req := ledger.Fixed(id)
		req.Amount = amount
		body, _ = json.Marshal(req)
	}
	req, _ := http.NewRequest(method, target+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Spiffe-ID", identity.Trusted.String())
	req.Header.Set("X-Forwarded-Spiffe-ID", identity.Trusted.String())
	response, err := client.Do(req)
	if err != nil {
		message := err.Error()
		if result.ServerVerified && (strings.Contains(message, "remote error: tls:") || strings.Contains(message, "tls: certificate required")) {
			result.Outcome = "tls_client_rejected"
		}
		if mismatch {
			result.Outcome = "server_identity_mismatch"
		}
		return result, err
	}
	defer response.Body.Close()
	result.Status = response.StatusCode
	result.Outcome = "money_http_response"
	data, err := io.ReadAll(io.LimitReader(response.Body, 8193))
	if err != nil || len(data) > 8192 {
		return result, fmt.Errorf("bounded response invalid")
	}
	if !json.Valid(data) {
		return result, fmt.Errorf("response JSON invalid")
	}
	result.Body = data
	return result, nil
}
func rotation(source *workloadapi.X509Source, target string) (report, error) {
	first, err := source.GetX509SVID()
	if err != nil {
		return report{}, err
	}
	initialSerial := first.Certificates[0].SerialNumber.String()
	expires := first.Certificates[0].NotAfter
	frozen, err := identity.ClientTLS(source, source, identity.Money).GetClientCertificate(nil)
	if err != nil {
		return report{}, err
	}
	if expires.Sub(time.Now()) > 70*time.Second {
		return report{}, fmt.Errorf("rotation fixture TTL must be short")
	}
	deadline := time.Now().Add(75 * time.Second)
	rotated := false
	for time.Now().Before(deadline) {
		current, err := source.GetX509SVID()
		if err != nil {
			return report{}, err
		}
		rotated = current.Certificates[0].SerialNumber.String() != initialSerial
		if rotated && time.Now().After(expires.Add(time.Second)) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !rotated {
		return report{}, fmt.Errorf("source did not rotate")
	}
	old := identity.ClientTLS(source, source, identity.Money)
	old.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return frozen, nil }
	rejected, _ := call(old, target, "transfer", "expired-credential", 100)
	current, err := call(identity.ClientTLS(source, source, identity.Money), target, "transfer", "rotation-current", 100)
	if err != nil {
		return current, err
	}
	checks := map[string]bool{"source_rotated": rotated, "expired_credential_rejected": rejected.Outcome == "tls_client_rejected" && rejected.Status == 0,
		"current_credential_accepted": current.Status == 200, "server_verified": current.ServerVerified && rejected.ServerVerified}
	config := identity.ClientTLS(source, source, identity.Money)
	config.ClientSessionCache = tls.NewLRUClientSessionCache(2)
	client, transport := identity.HTTPClient(config)
	transport.DisableKeepAlives = false
	defer transport.CloseIdleConnections()
	for i := 0; i < 2; i++ {
		body, _ := json.Marshal(ledger.Fixed("reuse-attempt"))
		response, err := client.Post(target+"/transfer", "application/json", bytes.NewReader(body))
		if err != nil {
			return report{}, err
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		checks[fmt.Sprint("fresh_nonresumed_", i)] = response.TLS != nil && !response.TLS.DidResume && response.TLS.Version == tls.VersionTLS13 && response.Close
	}
	for _, ok := range checks {
		if !ok {
			return report{Outcome: "rotation_failure", Checks: checks}, fmt.Errorf("rotation/reuse check failed")
		}
	}
	return report{Outcome: "rotation_expiry_and_reuse_passed", ServerVerified: checks["server_verified"], Checks: checks}, nil
}
func main() {
	target := flag.String("target", "https://money:8443", "private test target")
	mode := flag.String("mode", "registered", "registered, no-cert, self-signed, rotation")
	action := flag.String("action", "transfer", "transfer or balances")
	id := flag.String("id", "security-transfer", "stable request ID")
	amount := flag.Int64("amount", 100, "test amount")
	expected := flag.Int("status", 200, "expected money HTTP status")
	idle := flag.Bool("idle", false, "hold test container without credentials/listeners")
	impostor := flag.Bool("impostor", false, "serve test-only wrong server identity")
	listen := flag.String("listen", "127.0.0.1:8443", "impostor private listener")
	flag.Parse()
	if *idle {
		for {
			time.Sleep(time.Hour)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var source *workloadapi.X509Source
	var config *tls.Config
	var err error
	if *mode == "no-cert" || *mode == "self-signed" {
		bundle, e := x509bundle.Load(spiffeid.RequireTrustDomainFromString("poc.internal"), "/workload/bundle.pem")
		err = e
		if err == nil {
			config = identity.NoCertificateTLS(bundle, identity.Money)
		}
		if err == nil && *mode == "self-signed" {
			fake, e := fakeCertificate()
			err = e
			config.Certificates = []tls.Certificate{fake}
		}
	} else {
		source, err = identity.Source(ctx, "unix:///workload/api.sock")
		if err == nil {
			defer source.Close()
			config = identity.ClientTLS(source, source, identity.Money)
		}
	}
	if err != nil {
		json.NewEncoder(os.Stdout).Encode(report{Outcome: "identity_or_bundle_unavailable"})
		os.Exit(1)
	}
	if *impostor {
		l, err := net.Listen("tcp", *listen)
		if err != nil {
			os.Exit(1)
		}
		server := http.Server{TLSConfig: identity.ServerTLS(source, source), Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })}
		server.ServeTLS(l, "", "")
		return
	}
	var result report
	if *mode == "rotation" {
		result, err = rotation(source, *target)
	} else {
		result, err = call(config, *target, *action, *id, *amount)
	}
	json.NewEncoder(os.Stdout).Encode(result)
	if *mode == "rotation" {
		if err != nil {
			os.Exit(1)
		}
		return
	}
	if *mode == "no-cert" || *mode == "self-signed" {
		if result.Outcome != "tls_client_rejected" || result.Status != 0 {
			os.Exit(1)
		}
		return
	}
	if *expected == 0 {
		if result.Outcome != "server_identity_mismatch" {
			os.Exit(1)
		}
		return
	}
	if err != nil || result.Status != *expected {
		os.Exit(1)
	}
}
