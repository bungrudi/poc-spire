package controladapter

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"poc-spire/internal/httpjws"
)

type Remote struct {
	Client    *http.Client
	Transport *http.Transport
	Origin    string
}

func NewRemote(source *workloadapi.X509Source, role, origin string) (Remote, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || strings.Contains(u.Host, "\\") {
		return Remote{}, errors.New("exact HTTP control origin required")
	}
	server, err := ID(role)
	if err != nil {
		return Remote{}, err
	}
	client, transport := httpjws.Client(source, source, server)
	client.Timeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 10 * time.Second
	return Remote{Client: client, Transport: transport, Origin: origin}, nil
}
