// control-client is a local test harness, never an HTTP server or remote UI.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"poc-spire/internal/control"
	"time"
)

func main() {
	path := flag.String("socket", "/trusted-control/api.sock", "test control socket")
	id := flag.String("id", "control-request", "stable logical request ID")
	raw := flag.String("raw", "", "test malformed/extra-field protocol")
	idle := flag.Bool("idle", false, "hold private test harness container")
	flag.Parse()
	if *idle {
		for {
			time.Sleep(time.Hour)
		}
	}
	body, _ := json.Marshal(control.Request{RequestID: *id})
	if *raw != "" {
		body = []byte(*raw)
	}
	transport := &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", *path)
	}}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 12 * time.Second}
	response, err := client.Post("http://unix/attempt", "application/json", bytes.NewReader(body))
	if err != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"outcome": "control_unreachable"})
		os.Exit(1)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 8193))
	if response.StatusCode != 200 {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"control_status": response.StatusCode})
		os.Exit(1)
	}
	if len(data) > 8192 || !json.Valid(data) {
		fmt.Fprintln(os.Stderr, "invalid control result")
		os.Exit(1)
	}
	os.Stdout.Write(data)
}
