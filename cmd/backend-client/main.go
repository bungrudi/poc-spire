// backend-client is bounded stdin/stdout test instrumentation, with no listener.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"math/big"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"poc-spire/internal/caller"
	"poc-spire/internal/control"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
	"poc-spire/internal/signaturetest"
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
func request(svid *x509svid.SVID, bundle x509bundle.Source, target, id string, balances bool, at time.Time) response {
	result := response{Result: control.Result{RequestID: id, Stage: "payload_signature", Outcome: "unknown_commit"}}
	method, path := "POST", "/transfer"
	body, _ := json.Marshal(ledger.Fixed(id))
	if balances {
		method, path, body = "GET", "/balances", nil
	}
	verified, status, err := signaturetest.Exchange(context.Background(), svid, bundle, identity.Money, target, method, path, id, body, at)
	if err != nil {
		if strings.Contains(err.Error(), "signature") {
			result.Outcome = "server_trust_failure"
		}
		return result
	}
	result.ServerVerified = true
	result.MoneyStatus = status
	result.MoneyResult = verified.Body
	result.Stage = "money_http"
	result.Outcome = "money_http_response"
	// Authority is public certificate metadata, never signing material.
	result.ServerAuthority = verified.Authority
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
	target := flag.String("target", "http://money:8443", "private test target")
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
	var frozen *x509svid.SVID
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
				certificate, err := source.GetX509SVID()
				if err == nil {
					frozen = certificate
					result = sourceStatus(source)
				}
			}
		case "frozen-transfer":
			if source != nil && frozen != nil {
				at := time.Now()
				if !at.Before(frozen.Certificates[0].NotAfter) {
					at = frozen.Certificates[0].NotAfter.Add(-10 * time.Second)
				}
				result = request(frozen, source, *target, c.ID, false, at)
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
				svid, _ := source.GetX509SVID()
				result = request(svid, source, *target, c.ID, true, time.Now())
			}
		case "wrong-bundle":
			if source != nil {
				bundle, err := unrelatedBundle()
				if err == nil {
					svid, _ := source.GetX509SVID()
					result = request(svid, bundle, *target, c.ID, false, time.Now())
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
