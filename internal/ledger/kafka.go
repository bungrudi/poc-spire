package ledger

import (
	"errors"
	"time"
)

// GuardFailure distinguishes a terminal validity failure from a transient
// verification-budget or assignment failure. The guard must perform no I/O.
type GuardFailure struct {
	Reason    string
	Transient bool
}

func (e *GuardFailure) Error() string { return e.Reason }

type Receipt struct {
	InstanceID       string  `json:"instance_id"`
	Digest           string  `json:"digest"`
	ObservedAt       int64   `json:"observed_at"`
	RetainUntil      int64   `json:"retain_until"`
	Disposition      string  `json:"disposition"`
	Reason           string  `json:"reason"`
	VerifiedProducer string  `json:"verified_producer_id,omitempty"`
	RequestID        string  `json:"request_id,omitempty"`
	BusinessCode     int     `json:"business_code,omitempty"`
	BusinessResult   *Result `json:"business_result,omitempty"`
}
type KafkaMetadata struct {
	InstanceID    string `json:"instance_id"`
	StreamID      string `json:"stream_id"`
	ConsumerReady bool   `json:"consumer_ready"`
}
type kafkaState struct {
	metadata  KafkaMetadata
	epoch     uint64
	receipts  map[string]Receipt
	bindings  map[key]string
	capacity  int
	retention time.Duration
	clock     func() time.Time
}

func (l *Ledger) EnableKafka(instance, stream string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.kafka != nil || instance == "" || stream == "" {
		return errors.New("invalid_kafka_initialization")
	}
	l.kafka = &kafkaState{metadata: KafkaMetadata{InstanceID: instance, StreamID: stream}, receipts: map[string]Receipt{}, bindings: map[key]string{}, capacity: 4096, retention: 180 * time.Second, clock: time.Now}
	return nil
}

// SetKafkaAssignment shares the acceptance lock so revocation cannot race an
// accepted effect. A new epoch fences validation started on an older assignment.
func (l *Ledger) SetKafkaAssignment(ready bool) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.kafka == nil {
		return 0
	}
	l.kafka.epoch++
	l.kafka.metadata.ConsumerReady = ready
	return l.kafka.epoch
}
func (l *Ledger) KafkaEpoch() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.kafka == nil {
		return 0
	}
	return l.kafka.epoch
}
func (l *Ledger) KafkaMetadata() (KafkaMetadata, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.kafka == nil {
		return KafkaMetadata{}, false
	}
	return l.kafka.metadata, true
}
func cloneReceipt(r Receipt) Receipt {
	if r.BusinessResult != nil {
		result := clone(*r.BusinessResult)
		r.BusinessResult = &result
	}
	return r
}
func (l *Ledger) pruneReceipts(now time.Time) {
	for digest, r := range l.kafka.receipts {
		if now.Unix() >= r.RetainUntil {
			delete(l.kafka.receipts, digest)
		}
	}
}
func (l *Ledger) KafkaReceipt(digest string) (Receipt, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.kafka == nil {
		return Receipt{}, false
	}
	l.pruneReceipts(l.kafka.clock())
	r, ok := l.kafka.receipts[digest]
	return cloneReceipt(r), ok
}
func rejected(r Receipt) bool { return r.BusinessResult == nil }

// RejectedDigest can suppress only a retained no-effect rejection. A positive
// receipt is never permission to skip current signature and trust validation.
func (l *Ledger) RejectedDigest(digest string) (Receipt, bool) {
	r, ok := l.KafkaReceipt(digest)
	return r, ok && rejected(r)
}
func (l *Ledger) receipt(digest, disposition, reason, producer string, now time.Time) Receipt {
	return Receipt{InstanceID: l.kafka.metadata.InstanceID, Digest: digest, ObservedAt: now.Unix(), RetainUntil: now.Add(l.kafka.retention).Unix(), Disposition: disposition, Reason: reason, VerifiedProducer: producer}
}
func (l *Ledger) kafkaAvailable(digest string, now time.Time) *GuardFailure {
	if l.kafka == nil {
		return &GuardFailure{"kafka_disabled", true}
	}
	l.pruneReceipts(now)
	if _, exists := l.kafka.receipts[digest]; !exists && len(l.kafka.receipts) >= l.kafka.capacity {
		return &GuardFailure{"receipt_capacity_exhausted", true}
	}
	return nil
}

// RejectKafka records a deterministic no-effect result. The first receipt is
// immutable even if its digest is encountered after trust or time changes.
func (l *Ledger) RejectKafka(digest, reason, producer string, epoch uint64) (Receipt, *GuardFailure) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.kafka == nil {
		return Receipt{}, &GuardFailure{"kafka_disabled", true}
	}
	now := l.kafka.clock()
	if fail := l.kafkaAvailable(digest, now); fail != nil {
		return Receipt{}, fail
	}
	if !l.kafka.metadata.ConsumerReady || l.kafka.epoch != epoch {
		return Receipt{}, &GuardFailure{"assignment_lost", true}
	}
	if old, ok := l.kafka.receipts[digest]; ok {
		return cloneReceipt(old), nil
	}
	r := l.receipt(digest, "rejected", reason, producer, now)
	l.kafka.receipts[digest] = r
	return r, nil
}

// KafkaTransfer reserves capacity, guards validity and decides effects, context
// and receipt atomically. Sync transfers require no Kafka capacity or guards.
func (l *Ledger) KafkaTransfer(digest, caller, context string, request Request, epoch uint64, guard func() *GuardFailure) (Receipt, *GuardFailure) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.kafka == nil {
		return Receipt{}, &GuardFailure{"kafka_disabled", true}
	}
	now := l.kafka.clock()
	if fail := l.kafkaAvailable(digest, now); fail != nil {
		return Receipt{}, fail
	}
	if !l.kafka.metadata.ConsumerReady || l.kafka.epoch != epoch {
		return Receipt{}, &GuardFailure{"assignment_lost", true}
	}
	if old, ok := l.kafka.receipts[digest]; ok && rejected(old) {
		return cloneReceipt(old), nil
	}
	if guard == nil {
		return Receipt{}, &GuardFailure{"missing_acceptance_guard", true}
	}
	if fail := guard(); fail != nil {
		if fail.Transient {
			return Receipt{}, fail
		}
		if old, ok := l.kafka.receipts[digest]; ok {
			return cloneReceipt(old), nil
		}
		receipt := l.receipt(digest, "rejected", fail.Reason, caller, now)
		l.kafka.receipts[digest] = receipt
		return receipt, nil
	}
	k := key{caller, request.RequestID}
	if oldContext, ok := l.kafka.bindings[k]; ok && oldContext != context {
		r := l.receipt(digest, "conflicted", "immutable_context_conflict", caller, now)
		r.RequestID = request.RequestID
		r.BusinessCode = 409
		// This terminal decision has no history access or effect.
		if old, ok := l.kafka.receipts[digest]; ok {
			return cloneReceipt(old), nil
		}
		l.kafka.receipts[digest] = r
		return r, nil
	}
	if _, exists := l.kafka.bindings[k]; !exists && len(l.kafka.bindings) >= l.capacity {
		return Receipt{}, &GuardFailure{"context_capacity_exhausted", true}
	}
	result, status := l.transfer(caller, request)
	if status == 503 {
		return Receipt{}, &GuardFailure{result.Error, true}
	}
	l.kafka.bindings[k] = context
	disposition, reason := "applied", "transfer_applied"
	if result.Replay {
		disposition, reason = "replayed", "transfer_replayed"
	}
	if status != 200 {
		disposition, reason = "business_rejected", result.Error
		if status == 409 && result.Error == "idempotency_conflict" {
			disposition = "conflicted"
		}
	}
	r := l.receipt(digest, disposition, reason, caller, now)
	r.RequestID = request.RequestID
	r.BusinessCode = status
	r.BusinessResult = &result
	if old, ok := l.kafka.receipts[digest]; ok {
		return cloneReceipt(old), nil
	}
	l.kafka.receipts[digest] = cloneReceipt(r)
	return r, nil
}
