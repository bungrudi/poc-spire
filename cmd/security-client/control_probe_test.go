package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"poc-spire/internal/identity"
	"poc-spire/internal/signaturetest"
	"poc-spire/internal/testpki"
)

type fixtureControlSource struct {
	svid   *x509svid.SVID
	bundle *x509bundle.Bundle
}

func (s fixtureControlSource) GetX509SVID() (*x509svid.SVID, error) { return s.svid, nil }
func (s fixtureControlSource) GetX509BundleForTrustDomain(td spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return s.bundle.GetX509BundleForTrustDomain(td)
}

func TestFixedControlTargetsAndCasesCannotSelectArbitraryEndpoint(t *testing.T) {
	for _, role := range []string{"trusted", "unidentified"} {
		good, err := fixedControlCase("ui", role)
		if err != nil {
			t.Fatal(err)
		}
		wrong, err := fixedControlCase("wrong-server", role)
		if err != nil {
			t.Fatal(err)
		}
		if good.server != wrong.server || good.target == wrong.target || good.caller != identity.Observer {
			t.Fatal("wrong-server control lost exact expected identity")
		}
	}
	if _, err := fixedControlCase("arbitrary", "trusted"); err == nil {
		t.Fatal("arbitrary case allowed")
	}
	if _, err := fixedControlCase("ui", "https://foreign.invalid"); err == nil {
		t.Fatal("arbitrary target allowed")
	}
}

func TestFixedControlProbeSignedPositiveAndNegativeClassifications(t *testing.T) {
	ca, err := testpki.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	own := func(id spiffeid.ID) fixtureControlSource {
		svid, err := ca.SVID(id, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return fixtureControlSource{svid, ca.Bundle()}
	}
	ui := own(identity.Observer)
	wrong := own(spiffeid.RequireFromString("spiffe://poc.internal/test/control-client"))
	var calls atomic.Int32
	serve := func(role string) string {
		id := spiffeid.RequireFromString("spiffe://poc.internal/control/caller/" + role)
		source := own(id)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		server := &http.Server{Handler: signaturetest.Handler(source.svid, source.bundle, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/attempt/kafka/readiness" || r.URL.RawQuery != "" || r.Body == nil {
				t.Error("probe sent nonfixed operation")
			}
			body, _ := io.ReadAll(r.Body)
			if len(body) != 0 {
				t.Error("probe sent body")
			}
			peer := signaturetest.Peer(r)
			if peer != identity.Observer {
				w.WriteHeader(403)
				json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
				return
			}
			calls.Add(1)
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]any{"initialized": true, "exists": false, "request_id": "readiness"})
		}))}
		go server.Serve(listener)
		t.Cleanup(func() { server.Close() })
		return "http://" + listener.Addr().String() + "/attempt/kafka/readiness"
	}
	trusted, unidentified := serve("trusted"), serve("unidentified")
	for _, test := range []struct {
		name, url, outcome string
		source             controlMaterial
		public             x509bundle.Source
		status             int
	}{
		{"ui", trusted, "permitted_ui_read_verified", ui, nil, 404},
		{"wrong-id", trusted, "valid_wrong_identity_denied", wrong, nil, 403},
		{"no-cert", trusted, "signature_rejected", nil, ca.Bundle(), 401},
		{"wrong-server", unidentified, "server_identity_mismatch", ui, nil, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, err := fixedControlCase(test.name, "trusted")
			if err != nil {
				t.Fatal(err)
			}
			c.target = test.url
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			result, err := probeControl(ctx, c, test.source, test.public)
			if err != nil || result.Outcome != test.outcome || result.Status != test.status {
				t.Fatal(result, err)
			}
			if test.name != "wrong-server" && !result.ServerVerified {
				t.Fatal("outcome lacked independent exact-server verification")
			}
		})
	}
	// The wrong-server read is processed, then rejected by response verification.
	if calls.Load() != 2 {
		t.Fatal("unexpected permitted read count", calls.Load())
	}
	c, _ := fixedControlCase("ui", "trusted")
	c.target = trusted
	if _, err := probeControl(context.Background(), c, wrong, nil); err == nil {
		t.Fatal("UI probe silently borrowed wrong identity")
	}
	unrelated, err := testpki.New(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c, _ = fixedControlCase("no-cert", "trusted")
	c.target = trusted
	result, err := probeControl(context.Background(), c, nil, unrelated.Bundle())
	if err == nil || result.ServerVerified || result.Outcome == "signature_rejected" {
		t.Fatal("bad trust was counted as identity rejection", result, err)
	}
}
