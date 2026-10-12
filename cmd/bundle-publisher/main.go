// bundle-publisher reads public trust from private SPIRE administration and updates one owned ConfigMap.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const limit = 256 * 1024

func validBundle(data []byte, now time.Time) bool {
	if len(data) == 0 || len(data) > limit {
		return false
	}
	current := false
	count := 0
	for len(bytes.TrimSpace(data)) > 0 {
		if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("-----BEGIN CERTIFICATE-----")) {
			return false
		}
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return false
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !cert.IsCA || !cert.BasicConstraintsValid || cert.KeyUsage&x509.KeyUsageCertSign == 0 {
			return false
		}
		if !now.Before(cert.NotBefore) && now.Before(cert.NotAfter) {
			current = true
		}
		count++
		if count > 256 {
			return false
		}
		data = rest
	}
	return count > 0 && current
}

type configMap struct {
	Metadata struct {
		Name, Namespace, UID, ResourceVersion string
		Labels                                map[string]string
	} `json:"metadata"`
	Data map[string]string `json:"data"`
}

func fetch(ctx context.Context) ([]byte, error) {
	command := exec.CommandContext(ctx, "/opt/spire/bin/spire-server", "bundle", "show", "-socketPath", "/run/spire/admin/api.sock")
	pipe, err := command.StdoutPipe()
	if err != nil {
		return nil, errors.New("admin_unavailable")
	}
	if command.Start() != nil {
		return nil, errors.New("admin_unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(pipe, limit+1))
	if err != nil || len(data) > limit {
		command.Process.Kill()
		command.Wait()
		return nil, errors.New("bundle_unavailable")
	}
	if command.Wait() != nil || !validBundle(data, time.Now()) {
		return nil, errors.New("bundle_unavailable")
	}
	return data, nil
}

func refresh(ctx context.Context, client *http.Client, namespace, run string) error {
	bundle, err := fetch(ctx)
	if err != nil {
		return err
	}
	token, err := os.ReadFile("/run/publisher-api/token")
	if err != nil {
		return errors.New("api_unavailable")
	}
	target := "https://kubernetes.default.svc/api/v1/namespaces/" + namespace + "/configmaps/spire-bundle"
	call := func(method string, body []byte) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
		if err != nil {
			return nil, errors.New("api_unavailable")
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			return nil, errors.New("api_unavailable")
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, limit+16385))
		if err != nil || len(data) > limit+16384 || response.StatusCode != 200 {
			return nil, errors.New("api_unavailable")
		}
		return data, nil
	}
	data, err := call("GET", nil)
	if err != nil {
		return err
	}
	var cm configMap
	if json.Unmarshal(data, &cm) != nil || cm.Metadata.Name != "spire-bundle" || cm.Metadata.Namespace != namespace || cm.Metadata.UID == "" || cm.Metadata.ResourceVersion == "" || cm.Metadata.Labels["poc-run"] != run {
		return errors.New("ownership_mismatch")
	}
	if cm.Data["bundle.pem"] == string(bundle) {
		return nil
	}
	cm.Data = map[string]string{"bundle.pem": string(bundle)}
	body, err := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": cm.Metadata.Name, "namespace": cm.Metadata.Namespace, "uid": cm.Metadata.UID, "resourceVersion": cm.Metadata.ResourceVersion, "labels": cm.Metadata.Labels}, "data": cm.Data})
	if err != nil {
		return errors.New("api_unavailable")
	}
	_, err = call("PUT", body)
	return err
}

func main() {
	namespace := flag.String("namespace", "", "owned app namespace")
	run := flag.String("run", "", "expected run")
	flag.Parse()
	if *run == "" || *namespace != "poc-"+*run {
		os.Exit(1)
	}
	roots, err := os.ReadFile("/run/publisher-api/ca.crt")
	if err != nil {
		os.Exit(1)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(roots) {
		os.Exit(1)
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool}, DisableKeepAlives: true, TLSHandshakeTimeout: time.Second}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		operation, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := refresh(operation, client, *namespace, *run)
		cancel()
		if err != nil {
			os.Stdout.WriteString("public_bundle_refresh unavailable\n")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
