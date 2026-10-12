package controladapter

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

type Remote struct {
	Client    *http.Client
	Transport *http.Transport
	Origin    string
}

func NewRemote(source *workloadapi.X509Source, role, origin string) (Remote, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || strings.Contains(u.Host, "\\") {
		return Remote{}, errors.New("exact HTTPS control origin required")
	}
	client, transport, err := Client(source, role)
	if err != nil {
		return Remote{}, err
	}
	return Remote{Client: client, Transport: transport, Origin: origin}, nil
}
