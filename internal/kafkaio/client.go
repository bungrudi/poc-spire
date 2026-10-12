// Package kafkaio contains the opt-in private broker transport. Its TLS identity
// is separate from the SPIFFE identity validated on each signed command.
package kafkaio

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"poc-spire/internal/kafkacmd"
)

type Config struct {
	StreamID    string   `json:"stream_id"`
	Brokers     []string `json:"brokers"`
	CA          string   `json:"ca"`
	Certificate string   `json:"certificate"`
	Key         string   `json:"key"`
	StateDir    string   `json:"state_dir,omitempty"`
}

func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > 4096 {
		return Config{}, errors.New("broker_configuration_unavailable")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var cfg Config
	if decoder.Decode(&cfg) != nil {
		return Config{}, errors.New("invalid_broker_configuration")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return Config{}, errors.New("invalid_broker_configuration")
	}
	if !kafkacmd.Hex32.MatchString(cfg.StreamID) || len(cfg.Brokers) != 1 || cfg.Brokers[0] != "kafka:9093" || cfg.CA == "" || cfg.Certificate == "" || cfg.Key == "" {
		return Config{}, errors.New("invalid_broker_configuration")
	}
	return cfg, nil
}
func (c Config) Policy() kafkacmd.Policy { p, _ := kafkacmd.NewPolicy(c.StreamID); return p }
func (c Config) Group() string           { return "poc.spire.money." + c.StreamID }
func options(cfg Config) ([]kgo.Opt, error) {
	if !cfg.Policy().Valid() || len(cfg.Brokers) != 1 {
		return nil, errors.New("invalid_broker_configuration")
	}
	raw, err := os.ReadFile(cfg.CA)
	if err != nil || len(raw) > 8192 {
		return nil, errors.New("broker_trust_unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(raw) {
		return nil, errors.New("invalid_broker_trust")
	}
	pair, err := tls.LoadX509KeyPair(cfg.Certificate, cfg.Key)
	if err != nil {
		return nil, errors.New("broker_credential_unavailable")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{pair}, ServerName: "kafka"}
	return []kgo.Opt{kgo.SeedBrokers(cfg.Brokers...), kgo.DialTLSConfig(tlsConfig), kgo.DialTimeout(2 * time.Second), kgo.RequestTimeoutOverhead(time.Second), kgo.BrokerMaxReadBytes(131072), kgo.BrokerMaxWriteBytes(131072)}, nil
}
func Producer(cfg Config) (*kgo.Client, error) {
	opts, err := options(cfg)
	if err != nil {
		return nil, err
	}
	opts = append(opts, kgo.DisableIdempotentWrite(), kgo.RequiredAcks(kgo.AllISRAcks()), kgo.MaxProduceRequestsInflightPerBroker(1), kgo.MaxBufferedRecords(4), kgo.MaxBufferedBytes(65536), kgo.ProducerBatchCompression(kgo.NoCompression()), kgo.ProducerBatchMaxBytes(65536), kgo.RecordDeliveryTimeout(5*time.Second), kgo.ProduceRequestTimeout(2*time.Second))
	return kgo.NewClient(opts...)
}
func Publish(ctx context.Context, client *kgo.Client, record kafkacmd.Record) error {
	headers := make([]kgo.RecordHeader, len(record.Headers))
	for i, h := range record.Headers {
		headers[i] = kgo.RecordHeader{Key: h.Name, Value: h.Value}
	}
	r := &kgo.Record{Topic: record.Topic, Partition: 0, Key: record.Key, Value: record.Value, Headers: headers}
	return client.ProduceSync(ctx, r).FirstErr()
}
