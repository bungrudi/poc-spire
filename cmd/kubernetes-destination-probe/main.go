// kubernetes-destination-probe is controlled test instrumentation, never a trigger service.
package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"flag"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"poc-spire/internal/identity"
)

func main() {
	target := flag.String("target", "https://money:8443", "owned test destination")
	bundlePath := flag.String("bundle", "/control/probe-bundle.pem", "authentic public trust")
	negative := flag.Bool("expect-unreachable", false, "require failed destination verification despite local TCP acceptance")
	flag.Parse()
	result := map[string]any{"uid": os.Geteuid(), "gid": os.Getegid(), "destination_verified": false, "money_http_status": 0}
	ok := false
	defer func() {
		json.NewEncoder(os.Stdout).Encode(result)
		if !ok {
			os.Exit(1)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	api, err := workloadapi.New(ctx, workloadapi.WithAddr("unix:///workload/api.sock"))
	if err != nil {
		return
	}
	defer api.Close()
	svids, err := api.FetchX509SVIDs(ctx)
	if err == nil || len(svids) != 0 || status.Code(err) != codes.PermissionDenied || !strings.Contains(status.Convert(err).Message(), "no identity issued") {
		result["issuance"] = "not_proven_denied"
		return
	}
	result["issuance"] = "no_matching_identity"
	result["grpc_code"] = "PermissionDenied"
	bundle, err := x509bundle.Load(spiffeid.RequireTrustDomainFromString("poc.internal"), *bundlePath)
	if err != nil {
		result["outcome"] = "trust_unavailable"
		return
	}
	req, err := http.NewRequestWithContext(ctx, "GET", *target+"/balances", nil)
	if err != nil {
		return
	}
	connection, err := net.DialTimeout("tcp", req.URL.Host, time.Second)
	if err == nil {
		result["local_tcp_connectivity_only"] = true
		connection.Close()
	} else {
		result["local_tcp_connectivity_only"] = false
	}
	config := identity.NoCertificateTLS(bundle, identity.Money)
	var verified atomic.Bool
	verify := config.VerifyPeerCertificate
	config.VerifyPeerCertificate = func(raw [][]byte, chains [][]*x509.Certificate) error {
		err := verify(raw, chains)
		if err == nil {
			verified.Store(true)
		}
		return err
	}
	client, transport := identity.HTTPClient(config)
	defer transport.CloseIdleConnections()
	response, err := client.Do(req)
	result["destination_verified"] = verified.Load()
	if response != nil {
		result["money_http_status"] = response.StatusCode
		response.Body.Close()
		return
	}
	if *negative {
		ok = result["local_tcp_connectivity_only"] == true && result["destination_verified"] == false
		result["outcome"] = "destination_not_verified"
		return
	}
	ok = result["destination_verified"] == true && err != nil && strings.Contains(err.Error(), "remote error: tls: certificate required")
	if ok {
		result["outcome"] = "verified_money_no_certificate_rejected"
	} else {
		result["outcome"] = "infrastructure_unknown"
	}
}
