package kafkaio

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/ledger"
	"poc-spire/internal/testpki"
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
	onHeartbeat func()
	onCommit    func(*Delivery)
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
	if f.onHeartbeat != nil {
		f.onHeartbeat()
	}
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
	if f.onCommit != nil {
		f.onCommit(d)
	}
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

type changingTrust struct{ bundle *x509bundle.Bundle }

func (s *changingTrust) GetX509BundleForTrustDomain(spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	if s.bundle == nil {
		return nil, errors.New("offline")
	}
	return s.bundle, nil
}
func engineFixture(t *testing.T, capacity int) (*Engine, *changingTrust, kafkacmd.Record, *time.Time) {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	svid, err := ca.SVID(identity.Trusted, now)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := kafkacmd.NewPolicy("0123456789abcdef0123456789abcdef")
	r, fail := kafkacmd.Seal(svid, p.Context(now), ledger.Fixed("original"), now)
	if fail != nil {
		t.Fatal(fail)
	}
	book := ledger.New(capacity)
	if err := book.EnableKafka("instance", p.StreamID); err != nil {
		t.Fatal(err)
	}
	book.SetKafkaAssignment(true)
	source := &changingTrust{ca.Bundle()}
	return &Engine{Book: book, Policy: p, Source: source, Clock: func() time.Time { return now }}, source, r, &now
}
func TestValidDecisionLostAckReassignsWithSameIDAndCurrentGeneration(t *testing.T) {
	e, _, record, _ := engineFixture(t, 10)
	book := e.Book
	old := book.KafkaEpoch()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fakeTransport{book: book, cancel: cancel, ackFailures: 1, records: []*Delivery{{Record: record, Offset: 7, AssignmentEpoch: old, MemberID: "old-member", Generation: 2}}}
	attempts := 0
	f.onCommit = func(d *Delivery) {
		attempts++
		receipt, ok := book.KafkaReceipt(kafkacmd.Digest(record))
		if !ok || receipt.Disposition != "applied" || book.Snapshot().Version != 1 {
			t.Fatal("commit preceded valid ledger decision")
		}
		if attempts == 1 && (d.Generation != 2 || d.MemberID != "old-member") {
			t.Fatal("old delivery lost generation")
		}
		if attempts == 2 && (d.Generation != 3 || d.MemberID != "new-member") {
			t.Fatal("redelivery used stale member/generation")
		}
	}
	f.onHeartbeat = func() {
		if len(f.polls) != 1 || len(f.commits) != 1 {
			t.Fatal("later record passed uncertain ack")
		}
		book.SetKafkaAssignment(false)
		current := book.SetKafkaAssignment(true)
		// The same original broker bytes return under a newly captured assignment.
		f.records = []*Delivery{{Record: record, Offset: 7, AssignmentEpoch: current, MemberID: "new-member", Generation: 3}}
		f.onHeartbeat = nil
	}
	e.Run(ctx, f)
	if !reflect.DeepEqual(f.commits, []int64{8, 8}) || !reflect.DeepEqual(f.polls, []int64{7, 7}) || f.heartbeats != 1 || book.Snapshot().Version != 1 {
		t.Fatal("reassignment duplicated effect or advanced incorrectly", f.commits, f.polls)
	}
}
func TestMissingTrustPausesPendingValidRecordUntilSnapshotRecovers(t *testing.T) {
	e, source, record, _ := engineFixture(t, 10)
	bundle := source.bundle
	source.bundle = nil
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fakeTransport{book: e.Book, cancel: cancel, records: []*Delivery{{Record: record, Offset: 4, AssignmentEpoch: e.Book.KafkaEpoch()}}}
	f.onHeartbeat = func() {
		if len(f.commits) != 0 || e.Book.Snapshot().Version != 0 {
			t.Fatal("missing trust advanced")
		}
		if _, ok := e.Book.KafkaReceipt(kafkacmd.Digest(record)); ok {
			t.Fatal("missing trust created terminal receipt")
		}
		if _, status := e.Book.Transfer(identity.Trusted.String(), ledger.Fixed("sync-while-paused")); status != 200 {
			t.Fatal("paused consumer blocked Sync")
		}
		source.bundle = bundle
		f.onHeartbeat = nil
	}
	e.Run(ctx, f)
	if !reflect.DeepEqual(f.commits, []int64{5}) || len(f.polls) != 1 || f.heartbeats != 1 || e.Book.Snapshot().Version != 2 {
		t.Fatal("pending record not resumed exactly once", f.commits)
	}
}
func TestHistoryCapacityPausesWithoutPollingLaterRecordsOrBlockingReplay(t *testing.T) {
	e, _, record, _ := engineFixture(t, 1)
	e.Book.Transfer(identity.Trusted.String(), ledger.Fixed("existing"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fakeTransport{book: e.Book, cancel: cancel, records: []*Delivery{{Record: record, Offset: 1, AssignmentEpoch: e.Book.KafkaEpoch()}, {Record: record, Offset: 2, AssignmentEpoch: e.Book.KafkaEpoch()}}}
	f.onHeartbeat = func() {
		if len(f.commits) != 0 || len(f.polls) != 1 {
			t.Fatal("capacity allowed later progress")
		}
		if _, ok := e.Book.KafkaReceipt(kafkacmd.Digest(record)); ok {
			t.Fatal("capacity generated terminal receipt")
		}
		if r, status := e.Book.Transfer(identity.Trusted.String(), ledger.Fixed("existing")); status != 200 || !r.Replay {
			t.Fatal("capacity blocked existing Sync replay")
		}
		cancel()
	}
	e.Run(ctx, f)
	if e.Book.Snapshot().Version != 1 || f.heartbeats != 1 || len(f.commits) != 0 {
		t.Fatal("capacity evicted or mutated")
	}
}
func TestRejectedDigestRemainsImmutableWhenTimeOrTrustBecomesValid(t *testing.T) {
	for _, mode := range []string{"future-time", "initially-untrusted-root"} {
		t.Run(mode, func(t *testing.T) {
			e, source, record, now := engineFixture(t, 10)
			good := source.bundle
			original := *now
			reason := "invalid_message_time"
			if mode == "future-time" {
				*now = now.Add(-6 * time.Second)
			} else {
				other, err := testpki.New(*now)
				if err != nil {
					t.Fatal(err)
				}
				source.bundle = other.Bundle()
				reason = "untrusted_svid"
			}
			d := &Delivery{Record: record, AssignmentEpoch: e.Book.KafkaEpoch()}
			first := e.Decide(d)
			if !first.Terminal || first.Receipt == nil || first.Reason != reason || e.Book.Snapshot().Version != 0 {
				t.Fatal("first rejection", first)
			}
			*now = original
			source.bundle = good
			if _, fail := kafkacmd.Verify(record, e.Policy, source, *now); fail != nil {
				t.Fatal("fixture did not become valid", fail)
			}
			second := e.Decide(d)
			if !reflect.DeepEqual(first.Receipt, second.Receipt) || e.Book.Snapshot().Version != 0 {
				t.Fatal("retained rejected digest later applied")
			}
			// A freshly signed wire variant requires full current verification.
			ca, err := testpki.New(*now)
			if err != nil {
				t.Fatal(err)
			}
			svid, err := ca.SVID(identity.Trusted, *now)
			if err != nil {
				t.Fatal(err)
			}
			source.bundle = ca.Bundle()
			newRecord, fail := kafkacmd.Seal(svid, e.Policy.Context(*now), ledger.Fixed("original"), *now)
			if fail != nil {
				t.Fatal(fail)
			}
			fresh := e.Decide(&Delivery{Record: newRecord, AssignmentEpoch: e.Book.KafkaEpoch()})
			if !fresh.Terminal || fresh.Receipt.Disposition != "applied" || e.Book.Snapshot().Version != 1 {
				t.Fatal("fresh valid bytes did not receive full decision", fresh)
			}
			*now = now.Add(30 * time.Second)
			expired := e.Decide(&Delivery{Record: newRecord, AssignmentEpoch: e.Book.KafkaEpoch()})
			if !expired.Terminal || expired.Reason != "message_expired" || e.Book.Snapshot().Version != 1 {
				t.Fatal("positive receipt authorized expired bytes", expired)
			}
		})
	}
}
