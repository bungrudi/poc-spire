// backend-proxy is test-only opaque TLS-record forwarding. It has no credentials,
// Workload API access, TLS termination, or HTTP parsing. All listeners are private.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type counts struct {
	Forwarded int `json:"forwarded_records"`
	Dropped   int `json:"dropped_records"`
}

// The response-loss mode arms only when an encrypted client handshake record
// arrives. TLS remains opaque: the integration suite independently proves both
// server verification and a Money commit before accepting this fault as passed.
func relay(dst io.Writer, src io.Reader, dropAll bool, dropWhen, armOnEncrypted *atomic.Bool) (counts, error) {
	var result counts
	for {
		var header [5]byte
		if _, err := io.ReadFull(src, header[:]); err != nil {
			return result, err
		}
		size := int(binary.BigEndian.Uint16(header[3:]))
		if size > 18432 || header[0] < 20 || header[0] > 23 {
			return result, io.ErrUnexpectedEOF
		}
		record := make([]byte, 5+size)
		copy(record, header[:])
		if _, err := io.ReadFull(src, record[5:]); err != nil {
			return result, err
		}
		if header[0] == 23 && armOnEncrypted != nil {
			armOnEncrypted.Store(true)
		}
		if dropAll || (dropWhen != nil && dropWhen.Load() && header[0] == 23) {
			result.Dropped++
			continue
		}
		if _, err := io.Copy(dst, bytes.NewReader(record)); err != nil {
			return result, err
		}
		result.Forwarded++
	}
}

func run() int {
	upstream := flag.String("upstream", "money:8443", "private fixture destination")
	flag.Parse()
	modes := []struct{ address, mode string }{
		{":9443", "pass"}, {":9444", "discard-client"},
		{":9445", "discard-server"}, {":9446", "discard-server-after-handshake"},
	}
	var listeners []net.Listener
	for _, item := range modes {
		listener, err := net.Listen("tcp", item.address)
		if err != nil {
			for _, prior := range listeners {
				prior.Close()
			}
			return 1
		}
		listeners = append(listeners, listener)
	}
	var output sync.Mutex
	log := func(value any) { output.Lock(); defer output.Unlock(); json.NewEncoder(os.Stdout).Encode(value) }
	slots := make(chan struct{}, 16)
	for i, listener := range listeners {
		mode := modes[i].mode
		go func() {
			for {
				client, err := listener.Accept()
				if err != nil {
					return
				}
				select {
				case slots <- struct{}{}:
				default:
					client.Close()
					continue
				}
				go func() {
					defer func() { <-slots }()
					defer client.Close()
					server, err := net.DialTimeout("tcp", *upstream, 2*time.Second)
					if err != nil {
						log(map[string]any{"event": "upstream_unavailable", "mode": mode})
						return
					}
					defer server.Close()
					deadline := time.Now().Add(12 * time.Second)
					client.SetDeadline(deadline)
					server.SetDeadline(deadline)
					var responseLoss atomic.Bool
					var gate *atomic.Bool
					if mode == "discard-server-after-handshake" {
						gate = &responseLoss
					}
					type direction struct {
						Name   string
						Counts counts
					}
					done := make(chan direction, 2)
					go func() {
						c, _ := relay(server, client, mode == "discard-client", nil, gate)
						done <- direction{"client", c}
					}()
					go func() {
						c, _ := relay(client, server, mode == "discard-server", gate, nil)
						done <- direction{"server", c}
					}()
					first := <-done
					client.Close()
					server.Close()
					second := <-done
					log(map[string]any{"event": "connection_finished", "mode": mode, first.Name: first.Counts, second.Name: second.Counts})
				}()
			}
		}()
	}
	log(map[string]any{"event": "ready", "tls_terminated": false})
	select {}
}

func main() { os.Exit(run()) }
