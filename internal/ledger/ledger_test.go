package ledger

import (
	"fmt"
	"math"
	"sync"
	"testing"
)

func TestConcurrentRetryAndLostResponse(t *testing.T) {
	l := New(100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, status := l.Transfer("trusted", Fixed("one"))
			if status != 200 || r.Version != 1 {
				t.Errorf("bad retry: %+v %d", r, status)
			}
		}()
	}
	wg.Wait()
	if s := l.Snapshot(); s.Version != 1 || s.Balances["demo-a"] != 99900 || s.Balances["demo-b"] != 100100 {
		t.Fatal(s)
	}
	r, _ := l.Transfer("trusted", Fixed("one"))
	if !r.Replay {
		t.Fatal("unknown/lost result retry was applied again")
	}
	r.Balances["demo-a"] = 0
	again, _ := l.Transfer("trusted", Fixed("one"))
	if again.Balances["demo-a"] != 99900 {
		t.Fatal("response mutation corrupted history")
	}
}
func TestConflictIdentityBindingAndCapacity(t *testing.T) {
	l := New(2)
	l.Transfer("one", Fixed("id"))
	changed := Fixed("id")
	changed.Amount = 200
	if _, s := l.Transfer("one", changed); s != 409 {
		t.Fatal(s)
	}
	if r, s := l.Transfer("two", Fixed("id")); s != 200 || r.Replay {
		t.Fatal(r, s)
	}
	if _, s := l.Transfer("one", Fixed("new")); s != 503 {
		t.Fatal(s)
	}
	if r, s := l.Transfer("one", Fixed("id")); s != 200 || !r.Replay {
		t.Fatal("capacity forgot prior key")
	}
}
func TestBusinessRejectionsAreAtomicAndStable(t *testing.T) {
	for _, kind := range []string{"invalid", "insufficient", "overflow", "version"} {
		t.Run(kind, func(t *testing.T) {
			l := New(10)
			req := Fixed("bad")
			switch kind {
			case "invalid":
				req.Amount = -1
			case "insufficient":
				l.balances["demo-a"] = 0
			case "overflow":
				l.balances["demo-b"] = math.MaxInt64
			case "version":
				l.version = math.MaxUint64
			}
			before := l.Snapshot()
			r, s := l.Transfer("trusted", req)
			again, status := l.Transfer("trusted", req)
			if s == 200 || s != status || r.Error != again.Error || !again.Replay {
				t.Fatal(r, again, s, status)
			}
			after := l.Snapshot()
			if after.Version != before.Version || after.Balances["demo-a"] != before.Balances["demo-a"] || after.Balances["demo-b"] != before.Balances["demo-b"] {
				t.Fatal("partial mutation")
			}
		})
	}
}
func TestIndependentTransfersConserveMoney(t *testing.T) {
	l := New(100)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, s := l.Transfer("trusted", Fixed(fmt.Sprint(i))); s != 200 {
				t.Error(s)
			}
		}(i)
	}
	wg.Wait()
	s := l.Snapshot()
	if s.Version != 40 || s.Balances["demo-a"]+s.Balances["demo-b"] != 200000 {
		t.Fatal(s)
	}
}
