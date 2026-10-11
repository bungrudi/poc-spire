package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// Call cannot change the caller's destination, identity or fixed money payload.
// A lost control response is conservatively unknown, with the same ID on retry.
func Call(ctx context.Context, socket, id string) Result {
	result := Result{RequestID: id, Stage: "caller_control", Outcome: "unknown_commit"}
	transport := &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	body, _ := json.Marshal(Request{RequestID: id})
	req, err := http.NewRequestWithContext(ctx, "POST", "http://unix/attempt", bytes.NewReader(body))
	if err != nil {
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	reply, err := client.Do(req)
	if err != nil {
		var op *net.OpError
		if errors.As(err, &op) && op.Op == "dial" {
			result.Outcome = "infrastructure_failure"
		}
		return result
	}
	defer reply.Body.Close()
	if reply.StatusCode != http.StatusOK {
		result.Outcome = "infrastructure_failure"
		return result
	}
	data, err := io.ReadAll(io.LimitReader(reply.Body, 8193))
	if err != nil || len(data) > 8192 || json.Unmarshal(data, &result) != nil || result.RequestID != id {
		return Result{RequestID: id, Stage: "caller_control", Outcome: "unknown_commit"}
	}
	return result
}
