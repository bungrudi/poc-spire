package kafkaio

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/ledger"
)

type Delivery struct {
	Record          kafkacmd.Record
	Partition       int32
	Offset          int64
	LeaderEpoch     int32
	AssignmentEpoch uint64
	MemberID        string
	Generation      int32
}
type Transport interface {
	Poll(context.Context) (*Delivery, error)
	Pause()
	Resume()
	Heartbeat(context.Context) error
	Assigned(*Delivery) bool
	Commit(context.Context, *Delivery) error
}
type Engine struct {
	Book         *ledger.Ledger
	Policy       kafkacmd.Policy
	Source       x509bundle.Source
	Clock        func() time.Time
	ClockHealthy func() bool
	Audit        io.Writer
}
type Decision struct {
	Receipt    *ledger.Receipt
	Reason     string
	Terminal   bool
	VerifiedID string
}

func (e *Engine) now() time.Time {
	if e.Clock != nil {
		return e.Clock()
	}
	return time.Now()
}
func (e *Engine) Decide(delivery *Delivery) Decision {
	if e.ClockHealthy != nil && !e.ClockHealthy() {
		return Decision{Reason: "clock_unhealthy"}
	}
	r := delivery.Record
	// A fully available oversized record is deterministically rejected without
	// inventing an in-bounds receipt or touching business state.
	if len(r.Value) > kafkacmd.MaxValue || len(r.Key) > 128 {
		return Decision{Reason: "record_bounds", Terminal: true}
	}
	digest := kafkacmd.Digest(r)
	if old, ok := e.Book.RejectedDigest(digest); ok {
		receipt, fail := e.Book.RejectKafka(digest, old.Reason, old.VerifiedProducer, delivery.AssignmentEpoch)
		if fail != nil {
			return Decision{Reason: fail.Reason}
		}
		return Decision{Receipt: &receipt, Reason: receipt.Reason, Terminal: true}
	}
	verified, fail := kafkacmd.Verify(r, e.Policy, e.Source, e.now())
	reason, currentID := "", ""
	var receipt ledger.Receipt
	var guardFailure *ledger.GuardFailure
	if fail != nil {
		if fail.Transient {
			return Decision{Reason: fail.Reason}
		}
		reason, currentID = fail.Reason, fail.VerifiedID
		receipt, guardFailure = e.Book.RejectKafka(digest, fail.Reason, fail.VerifiedID, delivery.AssignmentEpoch)
	} else {
		contextBytes, _ := json.Marshal(verified.Message.Context)
		currentID = verified.Producer
		acceptanceGuard := verified.LedgerGuard(e.now, func() bool { return true })
		guard := func() *ledger.GuardFailure {
			failure := acceptanceGuard()
			if failure != nil {
				reason = failure.Reason
			}
			return failure
		}
		receipt, guardFailure = e.Book.KafkaTransfer(digest, verified.Producer, string(contextBytes), verified.Message.Transfer, delivery.AssignmentEpoch, guard)
	}
	if guardFailure != nil {
		return Decision{Reason: guardFailure.Reason, VerifiedID: currentID}
	}
	if reason == "" {
		reason = "validated_shared_ledger_decision"
	}
	return Decision{Receipt: &receipt, Reason: reason, Terminal: true, VerifiedID: currentID}
}
func (e *Engine) audit(d Decision) {
	if e.Audit == nil {
		return
	}
	value := map[string]any{"event": "money_command", "ingress": "kafka", "stage": d.Reason, "terminal": d.Terminal}
	if d.VerifiedID != "" {
		value["authenticated_original_producer_id"] = d.VerifiedID
	}
	if d.Receipt != nil {
		value["record_digest"] = d.Receipt.Digest
		value["receipt_verified_producer_id"] = d.Receipt.VerifiedProducer
		value["receipt_request_id"] = d.Receipt.RequestID
		value["receipt_disposition"] = d.Receipt.Disposition
		value["receipt_business_code"] = d.Receipt.BusinessCode
	}
	json.NewEncoder(e.Audit).Encode(value)
}

// Run holds one record through transient verification and uncertain commits.
// No later offset can be acknowledged until this terminal decision is acknowledged.
func (e *Engine) Run(ctx context.Context, transport Transport) {
	var pending *Delivery
	decided := false
	for ctx.Err() == nil {
		if pending == nil {
			transport.Resume()
			poll, cancel := context.WithTimeout(ctx, time.Second)
			record, err := transport.Poll(poll)
			cancel()
			if err != nil || record == nil {
				continue
			}
			pending = record
			decided = false
			transport.Pause()
		}
		if !transport.Assigned(pending) {
			pending = nil
			decided = false
			continue
		}
		if !decided {
			decision := e.Decide(pending)
			e.audit(decision)
			decided = decision.Terminal
		}
		if decided {
			commit, cancel := context.WithTimeout(ctx, 2*time.Second)
			err := transport.Commit(commit, pending)
			cancel()
			if err == nil {
				pending = nil
				decided = false
				continue
			}
		}
		// Polling while paused drives group events without fetching later work;
		// heartbeats also continue in the client's background group machinery.
		heartbeat, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		transport.Heartbeat(heartbeat)
		cancel()
	}
	transport.Pause()
}
