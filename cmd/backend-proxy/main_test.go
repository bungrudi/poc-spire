package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync/atomic"
	"testing"
)

func record(kind byte, value string) []byte {
	r := make([]byte, 5+len(value))
	r[0], r[1], r[2] = kind, 3, 3
	binary.BigEndian.PutUint16(r[3:], uint16(len(value)))
	copy(r[5:], value)
	return r
}

func TestOpaqueRelayAndDirectionalLoss(t *testing.T) {
	handshake, encrypted, response := record(22, "hello"), record(23, "encrypted-handshake"), record(23, "encrypted-response")
	input := bytes.Join([][]byte{handshake, encrypted, response}, nil)
	for _, tc := range []struct {
		name               string
		all                bool
		want               []byte
		forwarded, dropped int
	}{
		{"pass", false, input, 3, 0},
		{"all", true, nil, 0, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			got, err := relay(&output, bytes.NewReader(input), tc.all, nil, nil)
			if err != io.EOF || !bytes.Equal(output.Bytes(), tc.want) || got.Forwarded != tc.forwarded || got.Dropped != tc.dropped {
				t.Fatalf("relay mismatch: counts=%+v err=%v", got, err)
			}
		})
	}
}

func TestMalformedRecordNeverForwarded(t *testing.T) {
	for _, input := range [][]byte{{23, 3, 3, 255, 255}, {19, 3, 3, 0, 0}, {23, 3, 3, 0, 2, 1}} {
		var output bytes.Buffer
		got, err := relay(&output, bytes.NewReader(input), false, nil, nil)
		if err == nil || output.Len() != 0 || got.Forwarded != 0 {
			t.Fatal("malformed or oversized TLS record forwarded")
		}
	}
}

func TestResponseLossArmsOnClientEncryptedFlight(t *testing.T) {
	var gate atomic.Bool
	handshake := bytes.Join([][]byte{record(22, "server-hello"), record(23, "encrypted-handshake-part-1"), record(23, "encrypted-handshake-part-2")}, nil)
	var output bytes.Buffer
	got, err := relay(&output, bytes.NewReader(handshake), false, &gate, nil)
	if err != io.EOF || got.Forwarded != 3 || !bytes.Equal(output.Bytes(), handshake) || gate.Load() {
		t.Fatal("server handshake must pass unchanged before client flight")
	}
	client := record(23, "encrypted-client-flight")
	var upstream bytes.Buffer
	got, err = relay(&upstream, bytes.NewReader(client), false, nil, &gate)
	if err != io.EOF || !gate.Load() || !bytes.Equal(upstream.Bytes(), client) {
		t.Fatal("client encrypted flight must pass unchanged and arm downstream loss")
	}
	output.Reset()
	got, err = relay(&output, bytes.NewReader(record(23, "encrypted-response")), false, &gate, nil)
	if err != io.EOF || got.Dropped != 1 || output.Len() != 0 {
		t.Fatal("armed encrypted response was not discarded")
	}
}
