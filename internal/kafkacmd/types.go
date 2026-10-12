// Package kafkacmd defines the bounded SPIFFE-backed command profile. Broker
// admission credentials and delivery metadata never authorize business effects.
package kafkacmd

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"regexp"
	"time"

	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
)

const (
	Type        = "application/vnd.poc-spire.kafka-command.v1+jws"
	Environment = "poc-spire-isolated"
	CommandType = "fake.transfer.v1"
	MaxValue    = 32768
	MaxHeader   = 16384
	MaxPayload  = 4096
)

var (
	Hex32     = regexp.MustCompile(`^[0-9a-f]{32}$`)
	Hex64     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	RequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

type Policy struct{ StreamID, Topic string }

func NewPolicy(stream string) (Policy, error) {
	if !Hex32.MatchString(stream) {
		return Policy{}, errors.New("invalid_stream")
	}
	return Policy{stream, "poc.spire.commands." + stream}, nil
}
func (p Policy) Valid() bool {
	expected, err := NewPolicy(p.StreamID)
	return err == nil && expected == p
}
func Nonce() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

type Context struct {
	Version     int64  `json:"version"`
	Environment string `json:"environment"`
	StreamID    string `json:"stream_id"`
	Topic       string `json:"topic"`
	Audience    string `json:"audience"`
	Type        string `json:"type"`
	CreatedAt   int64  `json:"created_at"`
	Deadline    int64  `json:"deadline"`
}

func (p Policy) Context(now time.Time) Context {
	return Context{1, Environment, p.StreamID, p.Topic, identity.Money.String(), CommandType, now.Unix(), now.Unix() + 120}
}

type Message struct {
	Context   Context        `json:"context"`
	Transfer  ledger.Request `json:"transfer"`
	IssuedAt  int64          `json:"issued_at"`
	ExpiresAt int64          `json:"expires_at"`
}
type Header struct {
	Name  string
	Value []byte
}
type Record struct {
	Topic      string
	Key, Value []byte
	Headers    []Header
}

func frame(h hash.Hash, b []byte) {
	var n [8]byte
	if b == nil {
		binary.BigEndian.PutUint64(n[:], ^uint64(0))
	} else {
		binary.BigEndian.PutUint64(n[:], uint64(len(b)))
	}
	h.Write(n[:])
	h.Write(b)
}

// Digest covers every producer-supplied field, including nullness and ordered
// headers. Offset, partition and timestamp are deliberately absent.
func Digest(r Record) string {
	h := sha256.New()
	h.Write([]byte("poc-spire-kafka-record-v1\x00"))
	frame(h, []byte(r.Topic))
	frame(h, r.Key)
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(r.Headers)))
	h.Write(n[:])
	for _, v := range r.Headers {
		frame(h, []byte(v.Name))
		frame(h, v.Value)
	}
	frame(h, r.Value)
	return hex.EncodeToString(h.Sum(nil))
}

type Failure struct {
	Reason     string
	Transient  bool
	VerifiedID string
}

func (e *Failure) Error() string         { return e.Reason }
func failure(reason string) *Failure     { return &Failure{Reason: reason} }
func unavailable(reason string) *Failure { return &Failure{Reason: reason, Transient: true} }
