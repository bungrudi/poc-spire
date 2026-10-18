package caller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"poc-spire/internal/identity"
	"poc-spire/internal/ledger"
	"poc-spire/internal/money"
	"poc-spire/internal/signaturetest"
	"poc-spire/internal/testpki"
)

func TestLostSignedResponseStaysUnknownAndSameIDRecoveryAppliesOnce(t *testing.T) {
	now := time.Now()
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	moneySVID, _ := ca.SVID(identity.Money, now)
	callerSVID, _ := ca.SVID(identity.Trusted, now)
	book := ledger.New(10)
	base := money.Server(signaturetest.Material{SVID: moneySVID, Bundle: ca.Bundle()}, book, io.Discard).Handler
	var dropped atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !dropped.Swap(true) {
			captured := httptest.NewRecorder()
			base.ServeHTTP(captured, r)
			if captured.Code != 200 {
				t.Error("request was not accepted", captured.Code)
			}
			connection, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			connection.Close()
			return
		}
		base.ServeHTTP(w, r)
	}))
	defer server.Close()
	execute := Trusted(signaturetest.Material{SVID: callerSVID, Bundle: ca.Bundle()}, server.URL)
	lost := execute(context.Background(), "same-id")
	if lost.Outcome != "unknown_commit" || lost.ServerVerified || lost.MoneyStatus != 0 || lost.RequestID != "same-id" || book.Snapshot().Version != 1 {
		t.Fatal(lost, book.Snapshot())
	}
	retry := execute(context.Background(), "same-id")
	if retry.Outcome != "transfer_succeeded" || !retry.ServerVerified || retry.MoneyStatus != 200 || book.Snapshot().Version != 1 {
		t.Fatal(retry, book.Snapshot())
	}
}

func TestExpiredSigningCredentialFailsLocallyWithoutInventingMoneyEvidence(t *testing.T) {
	now := time.Now()
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	expired, err := ca.SVIDBetween(identity.Trusted, now.Add(-time.Minute), now.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	result := Trusted(signaturetest.Material{SVID: expired, Bundle: ca.Bundle()}, "http://127.0.0.1:1")(context.Background(), "same-id")
	if result.Outcome != "credential_not_current" || result.MoneyStatus != 0 || result.ServerVerified || result.RequestID != "same-id" {
		t.Fatal(result)
	}
}
