// kubernetes-broker-ready verifies the local broker's real TLS/Kafka response, never a raw TCP accept.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"io"
	"net"
	"os"
	"time"

	"github.com/twmb/franz-go/pkg/kmsg"
)

func ready() bool {
	file, err := os.Open("/pki/ca.pem")
	if err != nil {
		return false
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(raw) > 8192 {
		return false
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(raw) {
		return false
	}
	pair, err := tls.LoadX509KeyPair("/pki/broker.pem", "/pki/broker.pem")
	if err != nil {
		return false
	}
	connection, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", "127.0.0.1:9093", &tls.Config{MinVersion: tls.VersionTLS13, ServerName: "kafka", RootCAs: roots, Certificates: []tls.Certificate{pair}})
	if err != nil {
		return false
	}
	defer connection.Close()
	if err = connection.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return false
	}
	request := kmsg.NewPtrApiVersionsRequest()
	request.Version = 0
	// Kafka v0 request header: ApiVersions key, version, fixed correlation, empty client ID.
	payload := request.AppendTo([]byte{0, 18, 0, 0, 0, 0, 0, 1, 0, 0})
	frame := binary.BigEndian.AppendUint32(nil, uint32(len(payload)))
	frame = append(frame, payload...)
	if _, err = connection.Write(frame); err != nil {
		return false
	}
	size := make([]byte, 4)
	if _, err = io.ReadFull(connection, size); err != nil {
		return false
	}
	length := binary.BigEndian.Uint32(size)
	if length < 10 || length > 65536 {
		return false
	}
	body := make([]byte, length)
	if _, err = io.ReadFull(connection, body); err != nil || binary.BigEndian.Uint32(body[:4]) != 1 {
		return false
	}
	response := kmsg.NewPtrApiVersionsResponse()
	response.Version = 0
	return response.ReadFrom(body[4:]) == nil && response.ErrorCode == 0 && len(response.ApiKeys) > 0
}

func main() {
	if !ready() {
		os.Exit(1)
	}
}
