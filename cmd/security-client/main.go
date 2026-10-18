// security-client is test instrumentation, not an application trigger endpoint.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"io"
	"net"
	"net/http"
	"os"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
	"poc-spire/internal/signaturetest"
	"poc-spire/internal/testpki"
	"time"
)

type report struct {
	Outcome        string          `json:"outcome"`
	Status         int             `json:"money_http_status,omitempty"`
	ServerVerified bool            `json:"server_verified"`
	Body           json.RawMessage `json:"body,omitempty"`
	Checks         map[string]bool `json:"checks,omitempty"`
}

func call(svid *x509svid.SVID, bundle x509bundle.Source, target, action, id string, amount int64, at time.Time) (report, error) {
	result := report{Outcome: "transport_failure"}
	method, path := "GET", "/balances"
	var body []byte
	if action == "transfer" {
		method, path = "POST", "/transfer"
		request := ledger.Fixed(id)
		request.Amount = amount
		body, _ = json.Marshal(request)
	}
	verified, status, err := signaturetest.Exchange(context.Background(), svid, bundle, identity.Money, target, method, path, id, body, at)
	if err != nil {
		if err.Error() == "server_identity_mismatch" {
			result.Outcome = "server_identity_mismatch"
		}
		return result, err
	}
	result.ServerVerified = true
	result.Status = status
	result.Body = verified.Body
	result.Outcome = "money_http_response"
	if status == 401 {
		result.Outcome = "signature_rejected"
	}
	return result, nil
}
func rotation(source *workloadapi.X509Source, target string) (report, error) {
	first, err := source.GetX509SVID()
	if err != nil {
		return report{}, err
	}
	frozenAt := time.Now()
	expires := first.Certificates[0].NotAfter
	if time.Until(expires) > 70*time.Second {
		return report{}, fmt.Errorf("rotation fixture TTL must be short")
	}
	deadline := time.Now().Add(75 * time.Second)
	rotated := false
	for time.Now().Before(deadline) {
		current, err := source.GetX509SVID()
		if err != nil {
			return report{}, err
		}
		rotated = current.Certificates[0].SerialNumber.Cmp(first.Certificates[0].SerialNumber) != 0
		if rotated && time.Now().After(expires.Add(time.Second)) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	old, err := call(first, source, target, "transfer", "expired-credential", 100, frozenAt)
	if err != nil {
		return old, err
	}
	currentSVID, err := source.GetX509SVID()
	if err != nil {
		return report{}, err
	}
	current, err := call(currentSVID, source, target, "transfer", "rotation-current", 100, time.Now())
	if err != nil {
		return current, err
	}
	checks := map[string]bool{"source_rotated": rotated, "expired_credential_rejected": old.Status == 403, "current_credential_accepted": current.Status == 200, "server_verified": old.ServerVerified && current.ServerVerified}
	for i := 0; i < 2; i++ {
		svid, err := source.GetX509SVID()
		if err != nil {
			return report{}, err
		}
		r, err := call(svid, source, target, "transfer", "reuse-attempt", 100, time.Now())
		checks[fmt.Sprint("signed_retry_", i)] = err == nil && r.Status == 200 && r.ServerVerified
	}
	for _, ok := range checks {
		if !ok {
			return report{Outcome: "rotation_failure", Checks: checks}, fmt.Errorf("signature rotation/retry check failed")
		}
	}
	return report{Outcome: "rotation_expiry_and_retry_passed", ServerVerified: true, Checks: checks}, nil
}
func main() {
	controlCase := flag.String("control-case", "", "fixed test-only adapter case: ui, no-cert, wrong-id, wrong-server")
	controlRole := flag.String("control-role", "trusted", "fixed adapter selection")
	target := flag.String("target", "http://money:8443", "private test target")
	mode := flag.String("mode", "registered", "registered, no-cert, self-signed, rotation")
	action := flag.String("action", "transfer", "transfer or balances")
	id := flag.String("id", "security-transfer", "stable request ID")
	amount := flag.Int64("amount", 100, "test amount")
	expected := flag.Int("status", 200, "expected Money status")
	idle := flag.Bool("idle", false, "hold fixture without credentials/listeners")
	impostor := flag.Bool("impostor", false, "serve signed test-only wrong identity")
	listen := flag.String("listen", "127.0.0.1:8443", "private fixture listener")
	flag.Parse()
	if *controlCase != "" {
		runControlProbe(*controlCase, *controlRole)
		return
	}
	if *idle {
		for {
			time.Sleep(time.Hour)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var source *workloadapi.X509Source
	var svid *x509svid.SVID
	var bundle x509bundle.Source
	var err error
	if *mode == "no-cert" || *mode == "self-signed" {
		bundle, err = x509bundle.Load(spiffeid.RequireTrustDomainFromString("poc.internal"), "/workload/bundle.pem")
		if err == nil && *mode == "self-signed" {
			var ca *testpki.CA
			ca, err = testpki.New(time.Now())
			if err == nil {
				svid, err = ca.SVID(identity.Trusted, time.Now())
			}
		}
	} else {
		source, err = identity.Source(ctx, "unix:///workload/api.sock")
		if err == nil {
			defer source.Close()
			bundle = source
			svid, err = source.GetX509SVID()
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
		server := http.Server{Handler: signaturetest.Handler(svid, bundle, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) })), ReadHeaderTimeout: time.Second}
		server.Serve(l)
		return
	}
	var result report
	if *mode == "rotation" {
		result, err = rotation(source, *target)
	} else {
		result, err = call(svid, bundle, *target, *action, *id, *amount, time.Now())
	}
	json.NewEncoder(os.Stdout).Encode(result)
	if *mode == "rotation" {
		if err != nil {
			os.Exit(1)
		}
		return
	}
	if *mode == "no-cert" || *mode == "self-signed" {
		if err != nil || result.Status != 401 || !result.ServerVerified {
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
