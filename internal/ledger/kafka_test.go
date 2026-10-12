package ledger

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func kafkaBook(t *testing.T, capacity int) (*Ledger, uint64, *time.Time) {
	t.Helper()
	l := New(capacity)
	if err := l.EnableKafka("instance", "stream"); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1791780000, 0)
	l.kafka.clock = func() time.Time { return now }
	return l, l.SetKafkaAssignment(true), &now
}
func currentGuard() *GuardFailure { return nil }
func TestKafkaSharesSyncHistoryAndImmutableContext(t *testing.T) {
	for _, first := range []string{"sync", "kafka"} {
		t.Run(first, func(t *testing.T) {
			l, epoch, _ := kafkaBook(t, 10)
			req := Fixed("one")
			if first == "sync" {
				l.Transfer("trusted", req)
			} else {
				if _, f := l.KafkaTransfer("digest-a", "trusted", "context", req, epoch, currentGuard); f != nil {
					t.Fatal(f)
				}
			}
			receipt, f := l.KafkaTransfer("digest-b", "trusted", "context", req, epoch, currentGuard)
			if f != nil || receipt.Disposition != "replayed" || !receipt.BusinessResult.Replay {
				t.Fatal(receipt, f)
			}
			result, status := l.Transfer("trusted", req)
			if status != 200 || !result.Replay || l.Snapshot().Version != 1 {
				t.Fatal("cross-ingress double effect")
			}
			receipt, f = l.KafkaTransfer("digest-c", "trusted", "changed", req, epoch, currentGuard)
			if f != nil || receipt.Reason != "immutable_context_conflict" || l.Snapshot().Version != 1 {
				t.Fatal(receipt, f)
			}
			result, status = l.Transfer("trusted", req)
			if status != 200 || !result.Replay {
				t.Fatal("Kafka context interfered with Sync")
			}
			req.Amount = 200
			receipt, f = l.KafkaTransfer("digest-d", "trusted", "context", req, epoch, currentGuard)
			if f != nil || receipt.BusinessCode != 409 || receipt.BusinessResult.Error != "idempotency_conflict" {
				t.Fatal(receipt, f)
			}
		})
	}
}
func TestConcurrentKafkaAndSyncOneEffect(t *testing.T) {
	l, epoch, _ := kafkaBook(t, 100)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				l.Transfer("trusted", Fixed("same"))
			} else {
				l.KafkaTransfer("digest", "trusted", "context", Fixed("same"), epoch, currentGuard)
			}
		}(i)
	}
	wg.Wait()
	if l.Snapshot().Version != 1 {
		t.Fatal("concurrent duplicate")
	}
}
func TestKafkaGuardsDoNotPopulateHistoryOrContext(t *testing.T) {
	for _, mode := range []string{"expired", "budget", "assignment"} {
		t.Run(mode, func(t *testing.T) {
			l, epoch, _ := kafkaBook(t, 10)
			guard := func() *GuardFailure { return &GuardFailure{mode, mode != "expired"} }
			r, f := l.KafkaTransfer("d", "trusted", "c", Fixed("one"), epoch, guard)
			if mode == "expired" {
				if f != nil || r.Disposition != "rejected" {
					t.Fatal(r, f)
				}
			} else {
				if f == nil || !f.Transient {
					t.Fatal(f)
				}
				if _, ok := l.KafkaReceipt("d"); ok {
					t.Fatal("transient created terminal receipt")
				}
			}
			if len(l.history) != 0 || len(l.kafka.bindings) != 0 || l.Snapshot().Version != 0 {
				t.Fatal("failed guard touched business state")
			}
		})
	}
	l, epoch, _ := kafkaBook(t, 10)
	l.SetKafkaAssignment(false)
	if _, f := l.KafkaTransfer("d", "trusted", "c", Fixed("r"), epoch, currentGuard); f == nil || !f.Transient {
		t.Fatal("revocation ignored")
	}
	l.SetKafkaAssignment(true)
	if _, f := l.KafkaTransfer("d", "trusted", "c", Fixed("r"), epoch, currentGuard); f == nil || !f.Transient {
		t.Fatal("old epoch accepted after reassignment")
	}
}
func TestReceiptCapacityRetentionAndPositiveImmutability(t *testing.T) {
	l, epoch, now := kafkaBook(t, 10)
	l.kafka.capacity = 1
	applied, f := l.KafkaTransfer("positive", "trusted", "c", Fixed("one"), epoch, currentGuard)
	if f != nil {
		t.Fatal(f)
	}
	if _, f := l.KafkaTransfer("full", "trusted", "c", Fixed("two"), epoch, currentGuard); f == nil || !f.Transient {
		t.Fatal("full receipt allowed effect")
	}
	if _, status := l.Transfer("trusted", Fixed("sync")); status != 200 {
		t.Fatal("Kafka saturation blocked Sync")
	}
	immutable, f := l.RejectKafka("positive", "message_expired", "", epoch)
	if f != nil || !reflect.DeepEqual(immutable, applied) {
		t.Fatal("positive receipt overwritten")
	}
	*now = now.Add(179 * time.Second)
	if _, ok := l.KafkaReceipt("positive"); !ok {
		t.Fatal("early receipt expiry")
	}
	*now = now.Add(time.Second)
	if _, ok := l.KafkaReceipt("positive"); ok {
		t.Fatal("receipt did not expire")
	}
	r, f := l.KafkaTransfer("new-digest", "trusted", "c", Fixed("one"), epoch, currentGuard)
	if f != nil || r.Disposition != "replayed" || l.Snapshot().Version != 2 {
		t.Fatal("receipt expiry evicted history", r, f)
	}
}
func TestRetainedRejectionCannotLaterCommitAndDoesNotAuthenticatePositive(t *testing.T) {
	l, epoch, now := kafkaBook(t, 10)
	rejected, f := l.RejectKafka("rejected", "untrusted_svid", "", epoch)
	if f != nil {
		t.Fatal(f)
	}
	called := false
	r, f := l.KafkaTransfer("rejected", "trusted", "c", Fixed("one"), epoch, func() *GuardFailure { called = true; return nil })
	if f != nil || called || r != rejected || l.Snapshot().Version != 0 {
		t.Fatal("retained rejection later committed")
	}
	*now = now.Add(180 * time.Second)
	r, f = l.KafkaTransfer("rejected", "trusted", "c", Fixed("one"), epoch, currentGuard)
	if f != nil || r.Disposition != "applied" {
		t.Fatal("retention bound incorrect", r, f)
	}
	guarded := false
	r, f = l.KafkaTransfer("rejected", "trusted", "c", Fixed("one"), epoch, func() *GuardFailure { guarded = true; return &GuardFailure{"budget", true} })
	if !guarded || f == nil || !f.Transient {
		t.Fatal("positive receipt authorized replay without guard")
	}
}
func TestHistoryCapacityDoesNotLeaveBindingOrReceipt(t *testing.T) {
	l, epoch, _ := kafkaBook(t, 1)
	l.Transfer("trusted", Fixed("one"))
	if _, f := l.KafkaTransfer("d", "trusted", "c", Fixed("two"), epoch, currentGuard); f == nil || !f.Transient {
		t.Fatal(f)
	}
	if len(l.kafka.bindings) != 0 || len(l.kafka.receipts) != 0 {
		t.Fatal("capacity failure left state")
	}
}

func TestExactReceiptAndHistoryCapacityLimits(t *testing.T) {
	l, epoch, now := kafkaBook(t, 10000)
	positive, fail := l.KafkaTransfer("first", "trusted", "original-context", Fixed("original"), epoch, currentGuard)
	if fail != nil {
		t.Fatal(fail)
	}
	for i := 1; i < 4096; i++ {
		if _, fail := l.RejectKafka(fmt.Sprintf("negative-%d", i), "malformed_jws", "", epoch); fail != nil {
			t.Fatal("4096-slot boundary too small", i, fail)
		}
	}
	if len(l.kafka.receipts) != 4096 {
		t.Fatal("receipt count")
	}
	if _, fail := l.KafkaTransfer("overflow", "trusted", "c", Fixed("new"), epoch, currentGuard); fail == nil || fail.Reason != "receipt_capacity_exhausted" {
		t.Fatal("4097th receipt accepted", fail)
	}
	if _, bound := l.kafka.bindings[key{"trusted", "new"}]; bound {
		t.Fatal("capacity reserved context")
	}
	if first, ok := l.KafkaReceipt("first"); !ok || !reflect.DeepEqual(first, positive) {
		t.Fatal("fresh receipt evicted")
	}
	*now = now.Add(179 * time.Second)
	if _, ok := l.KafkaReceipt("first"); !ok {
		t.Fatal("180-second retention shortened")
	}
	*now = now.Add(time.Second)
	if _, ok := l.KafkaReceipt("first"); ok {
		t.Fatal("receipt persisted at expiry")
	}
	// Fill actual 10000-key history/context capacities with fixed business
	// rejections after the 1000 available successful transfers; no eviction.
	for i := 1; i < 10000; i++ {
		id := fmt.Sprintf("key-%d", i)
		r, fail := l.KafkaTransfer(fmt.Sprintf("digest-%d", i), "trusted", "c", Fixed(id), epoch, currentGuard)
		if fail != nil { // Receipts expire independently of persistent logical history.
			if fail.Reason != "receipt_capacity_exhausted" {
				t.Fatal(fail)
			}
			*now = now.Add(180 * time.Second)
			r, fail = l.KafkaTransfer(fmt.Sprintf("digest-%d", i), "trusted", "c", Fixed(id), epoch, currentGuard)
		}
		if fail != nil || r.BusinessResult == nil {
			t.Fatal("history ended before 10000", i, fail)
		}
	}
	if len(l.history) != 10000 || len(l.kafka.bindings) != 10000 {
		t.Fatal("exact history/context count")
	}
	if _, fail := l.KafkaTransfer("extra", "trusted", "c", Fixed("extra"), epoch, currentGuard); fail == nil || fail.Reason != "context_capacity_exhausted" {
		t.Fatal("10001st context accepted", fail)
	}
	*now = now.Add(180 * time.Second)
	replay, fail := l.KafkaTransfer("fresh-replay", "trusted", "original-context", Fixed("original"), epoch, currentGuard)
	if fail != nil || !replay.BusinessResult.Replay || l.Snapshot().Version != 1000 {
		t.Fatal("existing history failed at full capacity", replay, fail)
	}
	if len(l.history) != 10000 || len(l.kafka.bindings) != 10000 {
		t.Fatal("full replay evicted state")
	}
}

func TestOriginalKafkaBusinessRejectionReplaysAcrossIngresses(t *testing.T) {
	l, epoch, _ := kafkaBook(t, 10)
	request := Fixed("rejected-business")
	request.Amount = 200
	first, fail := l.KafkaTransfer("rejection-a", "trusted", "context", request, epoch, currentGuard)
	if fail != nil || first.Disposition != "business_rejected" || first.BusinessCode != 400 {
		t.Fatal("original business rejection", first, fail)
	}
	sync, status := l.Transfer("trusted", request)
	if status != 400 || !sync.Replay || sync.Error != "invalid_fixed_transfer" {
		t.Fatal("Sync failed stable rejected replay")
	}
	again, fail := l.KafkaTransfer("rejection-b", "trusted", "context", request, epoch, currentGuard)
	if fail != nil || again.BusinessCode != 400 || !again.BusinessResult.Replay || l.Snapshot().Version != 0 {
		t.Fatal("new Kafka bytes bypassed original rejection", again, fail)
	}
}
