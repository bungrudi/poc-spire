package ledger_test

import (
	"encoding/base64"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/cryptosigner"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/ledger"
	"poc-spire/internal/testpki"
)

func TestRealVerifiedGuardCrossesExpiryWhileWaitingForLedgerLock(t *testing.T) {
	for _, mode := range []string{"message", "certificate", "deadline", "budget"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now().Truncate(time.Second)
			ca, err := testpki.New(now)
			if err != nil {
				t.Fatal(err)
			}
			svid, err := ca.SVID(identity.Trusted, now)
			if err != nil {
				t.Fatal(err)
			}
			policy, _ := kafkacmd.NewPolicy("0123456789abcdef0123456789abcdef")
			message := kafkacmd.Message{Context: policy.Context(now), Transfer: ledger.Fixed("lock"), IssuedAt: now.Unix(), ExpiresAt: now.Unix() + 30}
			if mode == "message" {
				message.ExpiresAt = now.Unix() + 1
			}
			if mode == "deadline" {
				message.Context.CreatedAt = now.Unix() - 119
				message.Context.Deadline = now.Unix() + 1
				message.ExpiresAt = now.Unix() + 1
			}
			if mode == "certificate" {
				// A freshly issued short-lived test SVID remains an independently verified
				// path at the initial snapshot, then expires while the lock is held.
				short, err := ca.Cert(svid.PrivateKey.Public(), identity.Trusted, now.Add(-10*time.Minute+time.Second))
				if err != nil {
					t.Fatal(err)
				}
				svid.Certificates[0] = short
			}
			payload, _ := json.Marshal(message)
			signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: cryptosigner.Opaque(svid.PrivateKey)}, (&jose.SignerOptions{}).WithType(jose.ContentType(kafkacmd.Type)).WithHeader(jose.HeaderKey("x5c"), []string{base64.StdEncoding.EncodeToString(svid.Certificates[0].Raw)}))
			if err != nil {
				t.Fatal(err)
			}
			signed, err := signer.Sign(payload)
			if err != nil {
				t.Fatal(err)
			}
			compact, err := signed.CompactSerialize()
			if err != nil {
				t.Fatal(err)
			}
			record := kafkacmd.Record{Topic: policy.Topic, Key: []byte("lock"), Value: []byte(compact)}
			verified, fail := kafkacmd.Verify(record, policy, ca.Bundle(), now)
			if fail != nil {
				t.Fatal(fail)
			}
			book := ledger.New(10)
			book.EnableKafka("instance", policy.StreamID)
			epoch := book.SetKafkaAssignment(true)
			unlock := ledger.HoldForTest(book)
			var clock atomic.Int64
			clock.Store(now.UnixNano())
			began := make(chan struct{})
			done := make(chan struct{})
			var receipt ledger.Receipt
			var failure *ledger.GuardFailure
			go func() {
				close(began)
				receipt, failure = book.KafkaTransfer(kafkacmd.Digest(record), verified.Producer, "original", message.Transfer, epoch, verified.LedgerGuard(func() time.Time { return time.Unix(0, clock.Load()) }, func() bool { return true }))
				close(done)
			}()
			<-began
			select {
			case <-done:
				t.Fatal("ledger lock did not fence acceptance")
			default:
			}
			delta := time.Second
			if mode == "budget" {
				delta = 2*time.Second + time.Nanosecond
			}
			clock.Store(now.Add(delta).UnixNano())
			unlock()
			<-done
			if book.Snapshot().Version != 0 {
				t.Fatal("expired blocked command changed balances")
			}
			if mode == "budget" {
				if failure == nil || !failure.Transient {
					t.Fatal("budget failure terminal", failure)
				}
				if _, ok := book.KafkaReceipt(kafkacmd.Digest(record)); ok {
					t.Fatal("budget inserted terminal receipt")
				}
			} else if failure != nil || receipt.Disposition != "rejected" {
				t.Fatal("expiry not terminal", receipt, failure)
			}
			// A subsequently valid different context can use the same key; neither
			// history nor the immutable-context binding was populated by the failed guard.
			later, f := book.KafkaTransfer("new-digest", identity.Trusted.String(), "different", message.Transfer, epoch, func() *ledger.GuardFailure { return nil })
			if f != nil || later.Disposition != "applied" || later.BusinessResult.Replay {
				t.Fatal("failed guard populated history or context")
			}
		})
	}
}
