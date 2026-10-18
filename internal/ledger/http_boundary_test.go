package ledger_test

import (
	"encoding/json"
	"poc-spire/internal/httpjws"
	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
	"poc-spire/internal/testpki"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPSignatureExpiresWhileWaitingWithoutHistoryInsertion(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	svid, err := ca.SVID(identity.Trusted, now)
	if err != nil {
		t.Fatal(err)
	}
	req := ledger.Fixed("blocked")
	body, _ := json.Marshal(req)
	compact, err := httpjws.Sign(svid, "POST", "/transfer", req.RequestID, body, now)
	if err != nil {
		t.Fatal(err)
	}
	verified, fail := httpjws.Verify(compact, ca.Bundle(), now)
	if fail != nil {
		t.Fatal(fail)
	}
	book := ledger.New(1)
	unlock := ledger.HoldForTest(book)
	var clock atomic.Int64
	clock.Store(now.Unix())
	began, done := make(chan struct{}), make(chan struct{})
	var status int
	go func() {
		close(began)
		_, status = book.TransferCurrent(verified.Signer.String(), req, func() bool { return verified.Current(time.Unix(clock.Load(), 0)) })
		close(done)
	}()
	<-began
	select {
	case <-done:
		t.Fatal("lock did not fence transfer")
	default:
	}
	clock.Store(now.Add(31 * time.Second).Unix())
	unlock()
	<-done
	if status != 403 || book.Snapshot().Version != 0 {
		t.Fatal(status, book.Snapshot())
	}
	// A rejected expired request cannot consume the single history slot.
	if _, status = book.TransferCurrent(identity.Trusted.String(), ledger.Fixed("fresh"), func() bool { return true }); status != 200 {
		t.Fatal("expiry inserted history", status)
	}
}
