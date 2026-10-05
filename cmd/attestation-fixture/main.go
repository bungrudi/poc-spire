// attestation-fixture is a test-only Workload API client and reachability sentinel.
// The sentinel is not a money API, TLS endpoint, or authentication fallback.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var buildRole = "approved"

type outcome struct {
	Role       string   `json:"role"`
	UID        int      `json:"uid"`
	GID        int      `json:"gid"`
	Socket     bool     `json:"socket_reachable"`
	Network    bool     `json:"network_reachable"`
	Identities []string `json:"identities,omitempty"`
	Code       string   `json:"grpc_code,omitempty"`
	Result     string   `json:"result"`
}

func denied(err error) bool {
	// A timeout, missing socket, unavailable agent, or plugin error is never a
	// successful rejection. This exact SPIRE response means no matching entry.
	return status.Code(err) == codes.PermissionDenied &&
		strings.Contains(status.Convert(err).Message(), "no identity issued")
}

func main() {
	socket := flag.String("socket", "/workload/api.sock", "Workload API socket")
	target := flag.String("target", "trusted:8443", "test-only reachability sentinel")
	expected := flag.String("expected", "spiffe://poc.internal/caller/trusted", "exact expected identity")
	expectDenied := flag.Bool("expect-denied", false, "require a real no-matching-identity rejection")
	listen := flag.String("listen", "", "serve a test-only TCP reachability sentinel")
	flag.Parse()
	if *listen != "" {
		l, err := net.Listen("tcp", *listen)
		if err != nil {
			fmt.Fprintln(os.Stderr, "sentinel listen failed")
			os.Exit(1)
		}
		defer l.Close()
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}
	o := outcome{Role: buildRole, UID: os.Geteuid(), GID: os.Getegid()}
	code := 1
	defer func() { json.NewEncoder(os.Stdout).Encode(o); os.Exit(code) }()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	d := net.Dialer{Timeout: time.Second}
	c, err := d.DialContext(ctx, "unix", *socket)
	if err != nil {
		o.Result = "socket_unreachable"
		return
	}
	c.Close()
	o.Socket = true
	c, err = d.DialContext(ctx, "tcp", *target)
	if err != nil {
		o.Result = "network_unreachable"
		return
	}
	c.Close()
	o.Network = true
	client, err := workloadapi.New(ctx, workloadapi.WithAddr("unix://"+*socket))
	if err != nil {
		o.Result = "client_error"
		return
	}
	defer client.Close()
	svids, err := client.FetchX509SVIDs(ctx)
	if err != nil {
		o.Code = status.Code(err).String()
		if *expectDenied && denied(err) {
			o.Result = "no_matching_identity"
			code = 0
		} else {
			o.Result = "unexpected_api_error"
		}
		return
	}
	for _, s := range svids {
		o.Identities = append(o.Identities, s.ID.String())
	}
	if *expectDenied {
		o.Result = "unexpected_identity_issued"
		return
	}
	if len(o.Identities) != 1 || o.Identities[0] != *expected {
		o.Result = "wrong_identity"
		return
	}
	o.Result = "expected_identity_issued"
	code = 0
}
