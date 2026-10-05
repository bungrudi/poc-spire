// backend-client is bounded stdin/stdout test instrumentation, with no listener.
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"poc-spire/internal/caller"
	"poc-spire/internal/control"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

var buildMarker = "v1"

type command struct {
	Action string `json:"action"`
	ID     string `json:"request_id"`
}
type response struct {
	control.Result
	Expires         int64    `json:"expires_at,omitempty"`
	Serial          string   `json:"serial,omitempty"`
	Authority       string   `json:"authority_key_id,omitempty"`
	Bundle          string   `json:"bundle_sha256,omitempty"`
	Roots           int      `json:"bundle_roots,omitempty"`
	ServerAuthority string   `json:"server_authority_key_id,omitempty"`
	Code            string   `json:"grpc_code,omitempty"`
	Build           string   `json:"build_marker,omitempty"`
	Authorities     []string `json:"bundle_authority_ids,omitempty"`
}

// The unrelated authority is a valid CA, but it did not issue Money's SVID.
// Its private key exists only in this test process and is never serialized.
func unrelatedBundle() (*x509bundle.Bundle, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return x509bundle.FromX509Authorities(spiffeid.RequireTrustDomainFromString("poc.internal"), []*x509.Certificate{certificate}), nil
}

func sourceStatus(source *workloadapi.X509Source) response {
	s, err := source.GetX509SVID()
	if err != nil {
		return response{Result: control.Result{Stage: "workload_api", Outcome: "identity_unavailable"}}
	}
	bundle, err := source.GetX509BundleForTrustDomain(s.ID.TrustDomain())
	if err != nil {
		return response{Result: control.Result{Stage: "workload_api", Outcome: "bundle_unavailable"}}
	}
	roots := []string{}
	authorities := []string{}
	for _, cert := range bundle.X509Authorities() {
		authorities = append(authorities, hex.EncodeToString(cert.SubjectKeyId))
		fingerprint := sha256.Sum256(cert.Raw)
		roots = append(roots, hex.EncodeToString(fingerprint[:]))
	}
	sort.Strings(roots)
	fingerprint := sha256.Sum256([]byte(strings.Join(roots, "\n")))
	leaf := s.Certificates[0]
	return response{Result: control.Result{Stage: "workload_api", Outcome: "source_ready"}, Expires: leaf.NotAfter.Unix(), Serial: leaf.SerialNumber.String(), Authority: hex.EncodeToString(leaf.AuthorityKeyId), Bundle: hex.EncodeToString(fingerprint[:]), Roots: len(roots), Build: buildMarker, Authorities: authorities}
}
func request(config *tls.Config, target, id string, balances bool) response {
	result := response{Result: control.Result{RequestID: id, Stage: "connect", Outcome: "infrastructure_failure"}}
	verify := config.VerifyPeerCertificate
	failed := false
	config.VerifyPeerCertificate = func(raw [][]byte, chains [][]*x509.Certificate) error {
		err := verify(raw, chains)
		if err != nil {
			failed = true
			return err
		}
		result.ServerVerified = true
		leaf, err := x509.ParseCertificate(raw[0])
		if err == nil {
			result.ServerAuthority = hex.EncodeToString(leaf.AuthorityKeyId)
		}
		return err
	}
	client, transport := identity.HTTPClient(config)
	defer transport.CloseIdleConnections()
	method, path := "POST", "/transfer"
	body, _ := json.Marshal(ledger.Fixed(id))
	if balances {
		method, path, body = "GET", "/balances", nil
	}
	req, err := http.NewRequest(method, target+path, bytes.NewReader(body))
	if err != nil {
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	reply, err := client.Do(req)
	if err != nil {
		if failed {
			result.Stage = "server_verification"
			result.Outcome = "server_trust_failure"
		} else if result.ServerVerified && strings.Contains(err.Error(), "remote error: tls:") {
			result.Stage = "client_authentication"
			result.Outcome = "tls_rejected"
		} else if result.ServerVerified {
			result.Stage = "transfer_response"
			result.Outcome = "unknown_commit"
		}
		return result
	}
	defer reply.Body.Close()
	result.Stage = "money_http"
	result.MoneyStatus = reply.StatusCode
	data, err := io.ReadAll(io.LimitReader(reply.Body, 8193))
	if err != nil || len(data) > 8192 || !json.Valid(data) {
		result.Outcome = "unknown_commit"
		return result
	}
	result.MoneyResult = data
	result.Outcome = "money_http_response"
	return result
}
func fresh(socket string) response {
	result := response{Result: control.Result{Stage: "workload_api", Outcome: "infrastructure_failure"}}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	client, err := workloadapi.New(ctx, workloadapi.WithAddr("unix://"+socket))
	if err != nil {
		return result
	}
	defer client.Close()
	_, err = client.FetchX509SVIDs(ctx)
	if err == nil {
		result.Outcome = "identity_issued"
		return result
	}
	result.Code = status.Code(err).String()
	if status.Code(err) == codes.PermissionDenied && strings.Contains(status.Convert(err).Message(), "no identity issued") {
		result.Outcome = "no_matching_identity"
	}
	return result
}
func run() int {
	target := flag.String("target", "https://money:8443", "private test target")
	expected := flag.String("expected", identity.Trusted.String(), "exact test identity")
	unidentified := flag.Bool("unidentified", false, "test unregistered process without loading credentials")
	flag.Parse()
	var source *workloadapi.X509Source
	if !*unidentified {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		var err error
		source, err = identity.Source(ctx, "unix:///workload/api.sock")
		cancel()
		if err != nil {
			json.NewEncoder(os.Stdout).Encode(response{Result: control.Result{Stage: "workload_api", Outcome: "identity_unavailable"}})
			return 1
		}
		defer source.Close()
		s, err := source.GetX509SVID()
		if err != nil || s.ID.String() != *expected {
			json.NewEncoder(os.Stdout).Encode(response{Result: control.Result{Stage: "workload_api", Outcome: "wrong_identity"}})
			return 1
		}
	}
	var frozen *tls.Certificate
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for {
		var c command
		if err := decoder.Decode(&c); err != nil {
			if err == io.EOF {
				return 0
			}
			return 1
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		result := response{Result: control.Result{Outcome: "invalid_test_command"}}
		switch c.Action {
		case "capture":
			if source != nil {
				config := identity.ClientTLS(source, source, identity.Money)
				certificate, err := config.GetClientCertificate(nil)
				if err == nil {
					frozen = certificate
					result = sourceStatus(source)
				}
			}
		case "frozen-transfer":
			if source != nil && frozen != nil {
				config := identity.ClientTLS(source, source, identity.Money)
				config.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return frozen, nil }
				result = request(config, *target, c.ID, false)
			}
		case "fresh":
			result = fresh("/workload/api.sock")
		case "missing-socket":
			result.Result = caller.Unidentified("/workload/missing.sock", "/workload/bundle.pem", *target)(ctx, c.ID)
		case "missing-bundle":
			result.Result = caller.Unidentified("/workload/api.sock", "/workload/missing.pem", *target)(ctx, c.ID)
		case "unidentified":
			result.Result = caller.Unidentified("/workload/api.sock", "/workload/bundle.pem", *target)(ctx, c.ID)
		case "status":
			if source != nil {
				result = sourceStatus(source)
			}
		case "transfer":
			if source != nil {
				result.Result = caller.Trusted(source, *target)(ctx, c.ID)
			}
		case "balances":
			if source != nil {
				result = request(identity.ClientTLS(source, source, identity.Money), *target, c.ID, true)
			}
		case "wrong-bundle":
			if source != nil {
				bundle, err := unrelatedBundle()
				if err == nil {
					result = request(identity.ClientTLS(source, bundle, identity.Money), *target, c.ID, false)
				}
			}
		}
		cancel()
		if err := encoder.Encode(result); err != nil {
			return 1
		}
	}
}
func main() { os.Exit(run()) }
