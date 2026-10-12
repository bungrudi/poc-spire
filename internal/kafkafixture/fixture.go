//go:build kafkatest

// Package kafkafixture is a fixed-case, host-controlled acceptance CLI. It is
// absent from normal application builds and exposes no browser/UDS payload API.
package kafkafixture

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"os"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/cryptosigner"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/kafkaio"
	"poc-spire/internal/ledger"
	"poc-spire/internal/moneyreader"
)

var command = flag.String("fixture-case", "", "fixed acceptance case; test build only")
var config = flag.String("fixture-config", "", "own test broker configuration")
var requestID = flag.String("fixture-id", "fixture", "logical acceptance ID")
var digest = flag.String("fixture-digest", "", "exact receipt digest")

func Run(ctx context.Context, source *workloadapi.X509Source) bool {
	if *command == "" {
		return false
	}
	result := map[string]any{"case": *command}
	fail := func(reason string) { result["error"] = reason; json.NewEncoder(os.Stdout).Encode(result); os.Exit(1) }
	if *command == "metadata" || *command == "receipt" {
		observer := moneyreader.Observer{Target: "https://money:8443", SVID: source, Bundle: source}
		if *command == "metadata" {
			m, err := observer.Metadata(ctx)
			if err != nil {
				fail("metadata_unavailable")
			}
			json.NewEncoder(os.Stdout).Encode(m)
		} else {
			r, m, status, err := observer.Receipt(ctx, *digest)
			if err != nil {
				fail("receipt_unavailable")
			}
			result["query_http_status"] = status
			if status == 200 {
				result["receipt"] = r
			} else {
				result["metadata"] = m
			}
			json.NewEncoder(os.Stdout).Encode(result)
		}
		return true
	}
	cfg, err := kafkaio.LoadConfig(*config)
	if err != nil {
		fail("fixture_config_unavailable")
	}
	prepare := strings.HasPrefix(*command, "prepare-")
	fixedCase := strings.TrimPrefix(*command, "prepare-")
	if fixedCase == "publish-prepared" {
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 65537))
		if err != nil || len(raw) > 65536 {
			fail("prepared_record_bounds")
		}
		var record kafkacmd.Record
		if json.Unmarshal(raw, &record) != nil || record.Topic != cfg.Policy().Topic || len(record.Value) > kafkacmd.MaxValue || len(record.Key) > 128 || len(record.Headers) > 4 {
			fail("prepared_record_invalid")
		}
		client, err := kafkaio.Producer(cfg)
		if err != nil {
			fail("fixture_broker_unavailable")
		}
		defer client.Close()
		if kafkaio.Publish(ctx, client, record) != nil {
			fail("fixture_broker_publication_failed")
		}
		json.NewEncoder(os.Stdout).Encode(map[string]any{"digest": kafkacmd.Digest(record), "broker_queued": true})
		return true
	}
	now := time.Now()
	svid, err := source.GetX509SVID()
	if err != nil {
		fail("fixture_svid_unavailable")
	}
	policy := cfg.Policy()
	message := kafkacmd.Message{Context: policy.Context(now), Transfer: ledger.Fixed(*requestID), IssuedAt: now.Unix(), ExpiresAt: min(now.Unix()+30, svid.Certificates[0].NotAfter.Unix()-5)}
	header := map[jose.HeaderKey]any{jose.HeaderKey("typ"): kafkacmd.Type, jose.HeaderKey("x5c"): []string{base64.StdEncoding.EncodeToString(svid.Certificates[0].Raw)}}
	signerKey := svid.PrivateKey
	key := []byte(*requestID)
	headers := []kafkacmd.Header{}
	delay := time.Duration(0)
	switch fixedCase {
	case "valid":
	case "business-invalid":
		message.Transfer.Amount = 200
	case "wrong-type":
		message.Context.Type = "other.type"
	case "wrong-environment":
		message.Context.Environment = "other-environment"
	case "wrong-audience":
		message.Context.Audience = identity.Observer.String()
	case "wrong-stream":
		message.Context.StreamID = strings.Repeat("f", 32)
	case "wrong-topic":
		message.Context.Topic += ".other"
	case "wrong-key":
		key = []byte("different-key")
	case "headers":
		headers = []kafkacmd.Header{{Name: "X-Spiffe-Id", Value: []byte(identity.Trusted.String())}}
	case "future-five":
		message.IssuedAt += 5
		message.ExpiresAt = min(message.IssuedAt+30, message.Context.Deadline, svid.Certificates[0].NotAfter.Unix()-5)
	case "future-six":
		message.IssuedAt += 6
		message.ExpiresAt = min(message.IssuedAt+30, message.Context.Deadline, svid.Certificates[0].NotAfter.Unix()-5)
	case "expired-message":
		message.ExpiresAt = now.Unix() + 1
		delay = 2 * time.Second
	case "unknown-header":
		header[jose.HeaderKey("jku")] = "https://invalid.example/key"
	case "fake-root":
		caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		caT := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}
		caDER, e := x509.CreateCertificate(rand.Reader, caT, caT, &caKey.PublicKey, caKey)
		if e != nil {
			fail("fake_fixture_unavailable")
		}
		ca, _ := x509.ParseCertificate(caDER)
		leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		uri, _ := url.Parse(identity.Trusted.String())
		leafT := &x509.Certificate{SerialNumber: big.NewInt(2), URIs: []*url.URL{uri}, KeyUsage: x509.KeyUsageDigitalSignature, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}
		der, e := x509.CreateCertificate(rand.Reader, leafT, ca, &leafKey.PublicKey, caKey)
		if e != nil {
			fail("fake_fixture_unavailable")
		}
		signerKey = leafKey
		header[jose.HeaderKey("x5c")] = []string{base64.StdEncoding.EncodeToString(der), base64.StdEncoding.EncodeToString(caDER)}
	case "tampered", "stripped", "none", "hs256", "es384", "der":
	default:
		fail("unsupported_fixed_case")
	}
	opts := &jose.SignerOptions{ExtraHeaders: header}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: cryptosigner.Opaque(signerKey)}, opts)
	if err != nil {
		fail("fixture_signing_failed")
	}
	payload, _ := json.Marshal(message)
	signed, err := signer.Sign(payload)
	if err != nil {
		fail("fixture_signing_failed")
	}
	compact, err := signed.CompactSerialize()
	if err != nil {
		fail("fixture_signing_failed")
	}
	parts := strings.Split(compact, ".")
	switch fixedCase {
	case "tampered":
		message.Transfer.Amount = 200
		payload, _ = json.Marshal(message)
		parts[1] = base64.RawURLEncoding.EncodeToString(payload)
	case "stripped":
		parts[2] = ""
	case "none", "hs256", "es384":
		raw, _ := base64.RawURLEncoding.DecodeString(parts[0])
		alg := map[string]string{"none": "none", "hs256": "HS256", "es384": "ES384"}[fixedCase]
		parts[0] = base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(string(raw), "ES256", alg, 1)))
	case "der":
		sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		private, ok := signerKey.(*ecdsa.PrivateKey)
		if !ok {
			fail("fixture_key_incompatible")
		}
		der, e := ecdsa.SignASN1(rand.Reader, private, sum[:])
		if e != nil {
			fail("fixture_signing_failed")
		}
		parts[2] = base64.RawURLEncoding.EncodeToString(der)
	}
	record := kafkacmd.Record{Topic: policy.Topic, Key: key, Value: []byte(strings.Join(parts, ".")), Headers: headers}
	if delay > 0 {
		select {
		case <-ctx.Done():
			fail("fixture_cancelled")
		case <-time.After(delay):
		}
	}
	if prepare {
		// Only public command wire bytes leave this test-only signing process by
		// pipe. The UI has no broker credential/network; a separate host-controlled
		// publisher supplies admission and never changes original-signer identity.
		json.NewEncoder(os.Stdout).Encode(record)
		return true
	}
	client, err := kafkaio.Producer(cfg)
	if err != nil {
		fail("fixture_broker_unavailable")
	}
	defer client.Close()
	if err := kafkaio.Publish(ctx, client, record); err != nil {
		fail("fixture_broker_publication_failed")
	}
	result["digest"] = kafkacmd.Digest(record)
	result["request_id"] = *requestID
	result["expires_at"] = message.ExpiresAt
	result["broker_queued"] = true
	result["svid_not_after"] = svid.Certificates[0].NotAfter.Unix()
	json.NewEncoder(os.Stdout).Encode(result)
	return true
}
func Diagnostic() { fmt.Fprintln(os.Stderr, "fixed acceptance fixture unavailable") }
