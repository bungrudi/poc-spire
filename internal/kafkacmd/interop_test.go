package kafkacmd

import (
	"bufio"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
	"poc-spire/internal/testpki"
)

func TestIndependentJOSEBidirectionalES256(t *testing.T) {
	if os.Getenv("POC_JOSE_INTEROP") != "1" {
		t.Skip("explicit independent JOSE suite: POC_JOSE_INTEROP=1 with pinned local Node/dependencies")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node runtime missing")
	}
	script, err := filepath.Abs("../../scripts/jose_interop.mjs")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, script)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); cmd.Wait() }()
	encoder := json.NewEncoder(input)
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 1024), 65536)
	read := func(out any) {
		t.Helper()
		if !scanner.Scan() {
			t.Fatal("independent process ended")
		}
		if json.Unmarshal(scanner.Bytes(), out) != nil {
			t.Fatal("invalid independent response")
		}
	}
	var greeting struct {
		SPKI    string `json:"spki"`
		Version string `json:"version"`
	}
	read(&greeting)
	if greeting.Version != "6.1.0" {
		t.Fatal("wrong independent version")
	}
	now := time.Now()
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	svid, err := ca.SVID(identity.Trusted, now)
	if err != nil {
		t.Fatal(err)
	}
	p := testPolicy(t)
	record, f := Seal(svid, p.Context(now), ledger.Fixed("interop"), now)
	if f != nil {
		t.Fatal(f)
	}
	if err := encoder.Encode(map[string]any{"kind": "verify", "compact": string(record.Value)}); err != nil {
		t.Fatal(err)
	}
	var checked struct {
		Verified        bool   `json:"verified"`
		AlteredRejected bool   `json:"alteredRejected"`
		CurveRejected   bool   `json:"curveRejected"`
		RFCVerified     bool   `json:"rfcVerified"`
		Payload         string `json:"payload"`
	}
	read(&checked)
	if !checked.Verified || !checked.AlteredRejected || !checked.CurveRejected || !checked.RFCVerified {
		t.Fatal("independent codec check failed")
	}
	block, _ := pem.Decode([]byte(greeting.SPKI))
	if block == nil {
		t.Fatal("no public key")
	}
	public, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := ca.Cert(public, identity.Trusted, now)
	if err != nil {
		t.Fatal(err)
	}
	message := Message{p.Context(now), ledger.Fixed("reverse"), now.Unix(), now.Unix() + 30}
	payload, _ := json.Marshal(message)
	header := map[string]any{"alg": "ES256", "typ": Type, "x5c": []string{base64.StdEncoding.EncodeToString(certificate.Raw)}}
	if err := encoder.Encode(map[string]any{"kind": "sign", "header": header, "payload": base64.RawURLEncoding.EncodeToString(payload)}); err != nil {
		t.Fatal(err)
	}
	var reverse struct {
		Compact string `json:"compact"`
	}
	read(&reverse)
	result, fail := Verify(Record{Topic: p.Topic, Key: []byte("reverse"), Value: []byte(reverse.Compact)}, p, ca.Bundle(), now)
	if fail != nil || result.Producer != identity.Trusted.String() || result.Message.Transfer != ledger.Fixed("reverse") {
		t.Fatal("independent signature failed Go profile policy", fail)
	}
	encoder.Encode(map[string]string{"kind": "done"})
}
