package identity

import (
	"context"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	Money    = spiffeid.RequireFromString("spiffe://poc.internal/money")
	Trusted  = spiffeid.RequireFromString("spiffe://poc.internal/caller/trusted")
	Observer = spiffeid.RequireFromString("spiffe://poc.internal/ui")
)

func Source(ctx context.Context, address string) (*workloadapi.X509Source, error) {
	for {
		source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(address)))
		if err == nil {
			return source, nil
		}
		if status.Code(err) != codes.PermissionDenied || !strings.Contains(err.Error(), "no identity issued") {
			return nil, err
		}
		// Newly registered entries may not yet be synchronized to the agent.
		// Bound startup by ctx; missing policy never becomes a fallback identity.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Material supplies rotating signing credentials and independently trusted bundles.
type Material interface {
	x509svid.Source
	x509bundle.Source
}

// Current checks cached source material without equating a disconnected issuer watch with revocation.
func Current(source interface {
	x509svid.Source
	x509bundle.Source
}) bool {
	svid, err := source.GetX509SVID()
	if err != nil || svid == nil || len(svid.Certificates) == 0 {
		return false
	}
	now := time.Now()
	for _, cert := range svid.Certificates {
		if cert == nil || now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
			return false
		}
	}
	verifiedID, chains, err := x509svid.Verify(svid.Certificates, source, x509svid.WithTime(now))
	if err != nil || verifiedID != svid.ID {
		return false
	}
	for _, chain := range chains {
		valid := true
		for _, cert := range chain {
			if now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
				valid = false
				break
			}
		}
		if valid {
			return true
		}
	}
	return false
}
