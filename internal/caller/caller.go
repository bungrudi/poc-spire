package caller

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"poc-spire/internal/control"
	"poc-spire/internal/httpjws"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

func request(ctx context.Context, id, target string, config *tls.Config) control.Result {
	result := control.Result{RequestID: id, Stage: "connect", Outcome: "infrastructure_failure"}
	verify := config.VerifyPeerCertificate
	mismatch := false
	config.VerifyPeerCertificate = func(raw [][]byte, chains [][]*x509.Certificate) error {
		err := verify(raw, chains)
		if err == nil {
			result.ServerVerified = true
		} else if strings.HasPrefix(err.Error(), "unexpected ID ") {
			mismatch = true
		}
		return err
	}
	client, transport := identity.HTTPClient(config)
	defer transport.CloseIdleConnections()
	body, _ := json.Marshal(ledger.Fixed(id))
	req, _ := http.NewRequestWithContext(ctx, "POST", target+"/transfer", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		if mismatch {
			result.Outcome = "server_identity_mismatch"
			result.Stage = "server_verification"
			return result
		}
		if result.ServerVerified && strings.Contains(err.Error(), "remote error: tls:") {
			result.Outcome = "tls_rejected"
			result.Stage = "client_authentication"
			return result
		}
		if result.ServerVerified {
			result.Outcome = "unknown_commit"
			result.Stage = "transfer_response"
		}
		return result
	}
	defer response.Body.Close()
	result.MoneyStatus = response.StatusCode
	result.Stage = "money_http"
	data, err := io.ReadAll(io.LimitReader(response.Body, 8193))
	if err != nil || len(data) > 8192 || !json.Valid(data) {
		result.Outcome = "unknown_commit"
		return result
	}
	result.MoneyResult = data
	if response.StatusCode == 200 {
		result.Outcome = "transfer_succeeded"
	} else if response.StatusCode == 403 {
		result.Outcome = "authenticated_denied"
	} else {
		result.Outcome = "business_rejected"
	}
	return result
}
func Trusted(source *workloadapi.X509Source, target string) control.Executor {
	return func(ctx context.Context, id string) control.Result {
		return request(ctx, id, target, identity.ClientTLS(source, source, identity.Money))
	}
}
func TrustedSigned(source *workloadapi.X509Source, target string) control.Executor {
	return func(ctx context.Context, id string) control.Result {
		return signedRequest(ctx, id, target, source, source)
	}
}
func signedRequest(ctx context.Context, id, target string, svid x509svid.Source, bundle x509bundle.Source) control.Result {
	result := control.Result{RequestID: id, Stage: "connect", Outcome: "infrastructure_failure"}
	client, transport := httpjws.Client(svid, bundle, identity.Money)
	defer transport.CloseIdleConnections()
	body, _ := json.Marshal(ledger.Fixed(id))
	req, err := http.NewRequestWithContext(ctx, "POST", target+"/transfer", bytes.NewReader(body))
	if err != nil {
		return result
	}
	response, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "response_signature_rejected") {
			result.Outcome = "unknown_commit"
			result.Stage = "payload_signature"
		}
		return result
	}
	defer response.Body.Close()
	result.ServerVerified = true
	result.MoneyStatus = response.StatusCode
	result.Stage = "money_http"
	data, err := io.ReadAll(io.LimitReader(response.Body, 8193))
	if err != nil || len(data) > 8192 || !json.Valid(data) {
		result.Outcome = "unknown_commit"
		return result
	}
	result.MoneyResult = data
	switch response.StatusCode {
	case 200:
		result.Outcome = "transfer_succeeded"
	case 401:
		result.Outcome = "signature_rejected"
		result.Stage = "payload_signature"
	case 403:
		if strings.Contains(string(data), "credential_not_current") {
			result.Outcome = "credential_not_current"
			result.Stage = "payload_freshness"
		} else {
			result.Outcome = "authenticated_denied"
		}
	default:
		result.Outcome = "business_rejected"
	}
	return result
}
func Unidentified(socket, bundlePath, target string) control.Executor {
	return func(ctx context.Context, id string) control.Result {
		result := control.Result{RequestID: id, Stage: "workload_api", Outcome: "infrastructure_failure"}
		connection, err := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", socket)
		if err != nil {
			return result
		}
		connection.Close()
		client, err := workloadapi.New(ctx, workloadapi.WithAddr("unix://"+socket))
		if err != nil {
			return result
		}
		defer client.Close()
		fetch, stop := context.WithTimeout(ctx, 4*time.Second)
		svids, err := client.FetchX509SVIDs(fetch)
		stop()
		if err == nil {
			_ = svids
			result.Outcome = "unexpected_identity_issued"
			return result
		}
		if status.Code(err) != codes.PermissionDenied || !strings.Contains(status.Convert(err).Message(), "no identity issued") {
			return result
		}
		result.IdentityEligibility = "no_matching_identity"
		result.Stage = "server_trust_bundle"
		bundle, err := x509bundle.Load(spiffeid.RequireTrustDomainFromString("poc.internal"), bundlePath)
		if err != nil {
			return result
		}
		result = request(ctx, id, target, identity.NoCertificateTLS(bundle, identity.Money))
		result.IdentityEligibility = "no_matching_identity"
		if result.Outcome == "transfer_succeeded" || result.MoneyStatus != 0 {
			result.Outcome = "unexpected_money_http_response"
		}
		return result
	}
}

// UnidentifiedSigned reaches a mesh Money listener that has no application TLS.
// A missing signature is an HTTP 401 from Money. Mesh denial still has no HTTP status.
func UnidentifiedSigned(socket, bundlePath, target string) control.Executor {
	return func(ctx context.Context, id string) control.Result {
		result := control.Result{RequestID: id, Stage: "workload_api", Outcome: "infrastructure_failure"}
		connection, err := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", socket)
		if err != nil {
			return result
		}
		connection.Close()
		client, err := workloadapi.New(ctx, workloadapi.WithAddr("unix://"+socket))
		if err != nil {
			return result
		}
		defer client.Close()
		fetch, stop := context.WithTimeout(ctx, 4*time.Second)
		svids, err := client.FetchX509SVIDs(fetch)
		stop()
		if err == nil {
			_ = svids
			result.Outcome = "unexpected_identity_issued"
			return result
		}
		if status.Code(err) != codes.PermissionDenied || !strings.Contains(status.Convert(err).Message(), "no identity issued") {
			return result
		}
		result.IdentityEligibility = "no_matching_identity"
		result.Stage = "server_trust_bundle"
		bundle, err := x509bundle.Load(spiffeid.RequireTrustDomainFromString("poc.internal"), bundlePath)
		if err != nil {
			return result
		}
		body, _ := json.Marshal(ledger.Fixed(id))
		req, err := http.NewRequestWithContext(ctx, "POST", target+"/transfer", bytes.NewReader(body))
		if err != nil {
			return result
		}
		plain := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := plain.Do(req)
		if err != nil {
			result.Stage = "mesh_connect"
			return result
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(response.Body, httpjws.MaxValue+1))
		if err != nil || len(raw) > httpjws.MaxValue {
			result.Outcome = "unknown_commit"
			result.Stage = "payload_signature"
			return result
		}
		verified, fail := httpjws.Verify(string(raw), bundle, time.Now())
		if fail != nil || verified.Signer != identity.Money {
			result.Outcome = "unknown_commit"
			result.Stage = "payload_signature"
			result.MoneyStatus = response.StatusCode
			return result
		}
		result.ServerVerified = true
		result.MoneyStatus = response.StatusCode
		result.MoneyResult = verified.Body
		result.Stage = "payload_signature"
		if response.StatusCode == 401 {
			result.Outcome = "signature_rejected"
			return result
		}
		result.Outcome = "unexpected_money_http_response"
		return result
	}
}
