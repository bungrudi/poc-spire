package kafkaio

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/ledger"
)

type missingTrust struct{}

func (missingTrust) GetX509BundleForTrustDomain(spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return nil, errors.New("offline")
}

type fakeTransport struct {
	mu          sync.Mutex
	book        *ledger.Ledger
	cancel      context.CancelFunc
	epoch       uint64
	records     []*Delivery
	polls       []int64
	commits     []int64
	ackFailures int
	heartbeats  int
	paused      bool
	revoke      bool
}

func (f *fakeTransport) Poll(ctx context.Context) (*Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.paused {
		panic("fetch while paused")
	}
	if len(f.records) == 0 {
		f.cancel()
		return nil, ctx.Err()
	}
	r := f.records[0]
	f.records = f.records[1:]
	f.polls = append(f.polls, r.Offset)
	return r, nil
}
func (f *fakeTransport) Pause()  { f.paused = true }
func (f *fakeTransport) Resume() { f.paused = false }
func (f *fakeTransport) Heartbeat(context.Context) error {
	f.heartbeats++
	if !f.paused {
		panic("uncertain record not paused")
	}
	if f.revoke {
		f.book.SetKafkaAssignment(false)
		f.revoke = false
	}
	return nil
}
func (f *fakeTransport) Assigned(d *Delivery) bool {
	meta, _ := f.book.KafkaMetadata()
	return meta.ConsumerReady && f.book.KafkaEpoch() == d.AssignmentEpoch
}
func (f *fakeTransport) Commit(_ context.Context, d *Delivery) error {
	f.commits = append(f.commits, d.Offset+1)
	if f.ackFailures > 0 {
		f.ackFailures--
		return errors.New("ack_lost")
	}
	return nil
}
func TestTerminalInvalidRecordsCommitInOrderThroughLostAck(t *testing.T) {
	p, _ := kafkacmd.NewPolicy("0123456789abcdef0123456789abcdef")
	book := ledger.New(10)
	book.EnableKafka("instance", p.StreamID)
	epoch := book.SetKafkaAssignment(true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transport := &fakeTransport{book: book, cancel: cancel, epoch: epoch, ackFailures: 1}
	for i := int64(0); i < 2; i++ {
		transport.records = append(transport.records, &Delivery{Record: kafkacmd.Record{Topic: p.Topic, Key: []byte("same"), Value: []byte("unsigned")}, Offset: i, AssignmentEpoch: epoch})
	}
	engine := Engine{Book: book, Policy: p, Source: missingTrust{}}
	engine.Run(ctx, transport)
	if len(transport.commits) != 3 || transport.commits[0] != 1 || transport.commits[1] != 1 || transport.commits[2] != 2 || transport.heartbeats != 1 || book.Snapshot().Version != 0 {
		t.Fatal("ordered commits or effects", transport.commits, transport.heartbeats)
	}
	receipt, ok := book.KafkaReceipt(kafkacmd.Digest(transport.recordsRecord(p)))
	if !ok || receipt.VerifiedProducer != "" || receipt.Reason != "malformed_jws" {
		t.Fatal("invalid record needed trust or claimed identity", receipt)
	}
}
func (f *fakeTransport) recordsRecord(p kafkacmd.Policy) kafkacmd.Record {
	return kafkacmd.Record{Topic: p.Topic, Key: []byte("same"), Value: []byte("unsigned")}
}
func TestPausedRecordRevocationNeverAcknowledgesOldAssignment(t *testing.T) {
	p, _ := kafkacmd.NewPolicy("0123456789abcdef0123456789abcdef")
	book := ledger.New(10)
	book.EnableKafka("instance", p.StreamID)
	epoch := book.SetKafkaAssignment(true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transport := &fakeTransport{book: book, cancel: cancel, epoch: epoch, revoke: true, records: []*Delivery{{Record: kafkacmd.Record{Topic: p.Topic, Value: []byte("unsigned")}, AssignmentEpoch: epoch}}}
	engine := Engine{Book: book, Policy: p, ClockHealthy: func() bool { return false }}
	engine.Run(ctx, transport)
	if len(transport.commits) != 0 || transport.heartbeats != 1 || book.Snapshot().Version != 0 {
		t.Fatal("revoked or unhealthy record advanced")
	}
	if _, ok := book.KafkaReceipt(kafkacmd.Digest(kafkacmd.Record{Topic: p.Topic, Value: []byte("unsigned")})); ok {
		t.Fatal("clock-health failure generated receipt")
	}
}
func TestEngineRejectsBoundedInvalidRecordWithoutBusinessAttribution(t *testing.T) {
	p, _ := kafkacmd.NewPolicy("0123456789abcdef0123456789abcdef")
	book := ledger.New(10)
	book.EnableKafka("instance", p.StreamID)
	epoch := book.SetKafkaAssignment(true)
	engine := Engine{Book: book, Policy: p, Clock: func() time.Time { return time.Now() }}
	decision := engine.Decide(&Delivery{Record: kafkacmd.Record{Topic: p.Topic, Value: []byte("unsigned")}, AssignmentEpoch: epoch})
	if !decision.Terminal || decision.Receipt == nil || decision.Receipt.RequestID != "" || decision.Receipt.VerifiedProducer != "" || decision.Receipt.BusinessCode != 0 {
		t.Fatal(decision)
	}
	book.SetKafkaAssignment(false)
	decision = engine.Decide(&Delivery{Record: kafkacmd.Record{Topic: p.Topic, Value: []byte("unsigned")}, AssignmentEpoch: epoch})
	if decision.Terminal || decision.Reason != "assignment_lost" {
		t.Fatal("rejection memo ignored fencing", decision)
	}
}
