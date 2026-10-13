package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"poc-spire/internal/identity"
)

type controlMaterial interface {
	x509svid.Source
	x509bundle.Source
}
type controlCase struct {
	name, role, target string
	server, caller     spiffeid.ID
}
type controlReport struct {
	Case           string `json:"case"`
	Role           string `json:"role"`
	Outcome        string `json:"outcome"`
	Caller         string `json:"caller_id,omitempty"`
	ExpectedServer string `json:"expected_server_id"`
	ServerVerified bool   `json:"server_verified"`
	Status         int    `json:"http_status"`
}

func fixedControlCase(name, role string) (controlCase, error) {
	if role != "trusted" && role != "unidentified" {
		return controlCase{}, errors.New("invalid fixed control role")
	}
	if name != "ui" && name != "no-cert" && name != "wrong-id" && name != "wrong-server" {
		return controlCase{}, errors.New("invalid fixed control case")
	}
	c := controlCase{name: name, role: role, server: spiffeid.RequireFromString("spiffe://poc.internal/control/caller/" + role), caller: identity.Observer}
	destination := role
	if name == "wrong-server" {
		destination = "trusted"
		if role == "trusted" {
			destination = "unidentified"
		}
	}
	host := "trusted-control"
	if destination == "unidentified" {
		host = "unidentified-control"
	}
	c.target = "https://" + host + ":9001/attempt/kafka/readiness"
	if name == "wrong-id" {
		c.caller = spiffeid.RequireFromString("spiffe://poc.internal/test/control-client")
	}
	return c, nil
}

func probeControl(ctx context.Context, c controlCase, source controlMaterial, public x509bundle.Source) (controlReport, error) {
	result := controlReport{Case: c.name, Role: c.role, ExpectedServer: c.server.String(), Outcome: "infrastructure_unknown"}
	var config *tls.Config
	if c.name == "no-cert" {
		if public == nil {
			return result, errors.New("public trust unavailable")
		}
		config = identity.NoCertificateTLS(public, c.server)
	} else {
		if source == nil {
			return result, errors.New("own fixture source unavailable")
		}
		svid, err := source.GetX509SVID()
		if err != nil || svid == nil || svid.ID != c.caller || !identity.Current(source) {
			return result, errors.New("own fixture identity unavailable")
		}
		result.Caller = svid.ID.String()
		config = identity.ClientTLS(source, source, c.server)
	}
	var verified, mismatch atomic.Bool
	verify := config.VerifyPeerCertificate
	config.VerifyPeerCertificate = func(raw [][]byte, chains [][]*x509.Certificate) error {
		err := verify(raw, chains) // The existing SDK verification/authorization always runs unchanged.
		if err == nil {
			verified.Store(true)
		} else if strings.Contains(err.Error(), "unexpected ID ") {
			mismatch.Store(true)
		}
		return err
	}
	client, transport := identity.HTTPClient(config)
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, "GET", c.target, nil)
	if err != nil {
		return result, err
	}
	response, err := client.Do(request)
	result.ServerVerified = verified.Load()
	if response == nil {
		if c.name == "no-cert" && result.ServerVerified && err != nil && strings.Contains(err.Error(), "remote error: tls: certificate required") {
			result.Outcome = "no_certificate_rejected"
			return result, nil
		}
		if c.name == "wrong-server" && !result.ServerVerified && mismatch.Load() && err != nil {
			result.Outcome = "server_identity_mismatch"
			return result, nil
		}
		return result, errors.New("adapter TLS result unproven")
	}
	defer response.Body.Close()
	result.Status = response.StatusCode
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 8193))
	if readErr != nil || len(body) > 8192 {
		return result, errors.New("bounded adapter response unavailable")
	}
	if c.name == "wrong-id" && result.ServerVerified && result.Status == 403 {
		var denial struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &denial) == nil && denial.Error == "forbidden" {
			result.Outcome = "valid_wrong_identity_denied"
			return result, nil
		}
	}
	if c.name == "ui" && result.ServerVerified && result.Status == 404 {
		var metadata struct {
			Initialized bool   `json:"initialized"`
			Exists      bool   `json:"exists"`
			RequestID   string `json:"request_id"`
		}
		if json.Unmarshal(body, &metadata) == nil && metadata.Initialized && !metadata.Exists && metadata.RequestID == "readiness" {
			result.Outcome = "permitted_ui_read_verified"
			return result, nil
		}
	}
	return result, errors.New("fixed adapter outcome differs")
}

func runControlProbe(name, role string) {
	c, err := fixedControlCase(name, role)
	if err != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"outcome": "invalid_fixed_control_case"})
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var source controlMaterial
	var public x509bundle.Source
	if name == "no-cert" {
		public, err = x509bundle.Load(spiffeid.RequireTrustDomainFromString("poc.internal"), "/trust/spire/bundle.pem")
	} else {
		own, e := identity.Source(ctx, "unix:///workload/api.sock")
		err = e
		if own != nil {
			defer own.Close()
			source = own
		}
	}
	if err != nil {
		json.NewEncoder(os.Stdout).Encode(controlReport{Case: name, Role: role, ExpectedServer: c.server.String(), Outcome: "identity_or_trust_unavailable"})
		os.Exit(1)
	}
	result, err := probeControl(ctx, c, source, public)
	json.NewEncoder(os.Stdout).Encode(result)
	if err != nil {
		os.Exit(1)
	}
}
