package identity

import (
	"context"
	"crypto/tls"
	"net/http"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
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

func hardened(config *tls.Config) *tls.Config {
	config.MinVersion = tls.VersionTLS13
	config.SessionTicketsDisabled = true
	config.ClientSessionCache = nil
	config.NextProtos = []string{"http/1.1"}
	return config
}

func ServerTLS(svid x509svid.Source, bundle x509bundle.Source) *tls.Config {
	// SDK verification authenticates the SVID; exact route permission is applied
	// separately in the money handler. No application verification bypass.
	return hardened(tlsconfig.MTLSServerConfig(svid, bundle,
		tlsconfig.AuthorizeMemberOf(spiffeid.RequireTrustDomainFromString("poc.internal"))))
}

func ClientTLS(svid x509svid.Source, bundle x509bundle.Source, server spiffeid.ID) *tls.Config {
	return hardened(tlsconfig.MTLSClientConfig(svid, bundle, tlsconfig.AuthorizeID(server)))
}

func NoCertificateTLS(bundle x509bundle.Source, server spiffeid.ID) *tls.Config {
	return hardened(tlsconfig.TLSClientConfig(bundle, tlsconfig.AuthorizeID(server)))
}

func HTTPClient(config *tls.Config) (*http.Client, *http.Transport) {
	transport := &http.Transport{TLSClientConfig: config, DisableKeepAlives: true,
		ForceAttemptHTTP2: false, TLSHandshakeTimeout: 3 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second, MaxConnsPerHost: 8}
	return &http.Client{Transport: transport, Timeout: 8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, transport
}

// Current checks cached source material without equating a disconnected issuer watch with revocation.
func Current(source *workloadapi.X509Source) bool {
	svid, err := source.GetX509SVID()
	if err != nil || len(svid.Certificates) == 0 {
		return false
	}
	now := time.Now()
	for _, cert := range svid.Certificates {
		if now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
			return false
		}
	}
	_, err = source.GetX509BundleForTrustDomain(svid.ID.TrustDomain())
	return err == nil
}
