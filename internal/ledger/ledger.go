package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"regexp"
	"sync"
)

type Request struct {
	RequestID string `json:"request_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Amount    int64  `json:"amount_minor"`
	Currency  string `json:"currency"`
}
type Snapshot struct {
	Balances map[string]int64 `json:"balances"`
	Version  uint64           `json:"version"`
}
type Result struct {
	RequestID  string `json:"request_id"`
	TransferID string `json:"transfer_id,omitempty"`
	Caller     string `json:"verified_caller_id"`
	Amount     int64  `json:"amount_minor"`
	Currency   string `json:"currency"`
	Snapshot
	Replay bool   `json:"replay"`
	Error  string `json:"error,omitempty"`
}
type key struct{ caller, id string }
type record struct {
	request Request
	result  Result
	status  int
}
type Ledger struct {
	mu       sync.Mutex
	balances map[string]int64
	version  uint64
	history  map[key]record
	capacity int
}

var requestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func New(capacity int) *Ledger {
	return &Ledger{balances: map[string]int64{"demo-a": 100000, "demo-b": 100000},
		history: make(map[key]record), capacity: capacity}
}
func Fixed(id string) Request { return Request{id, "demo-a", "demo-b", 100, "USD"} }
func (l *Ledger) snapshot() Snapshot {
	return Snapshot{map[string]int64{"demo-a": l.balances["demo-a"], "demo-b": l.balances["demo-b"]}, l.version}
}
func (l *Ledger) Snapshot() Snapshot { l.mu.Lock(); defer l.mu.Unlock(); return l.snapshot() }
func clone(r Result) Result {
	r.Balances = map[string]int64{"demo-a": r.Balances["demo-a"], "demo-b": r.Balances["demo-b"]}
	return r
}

func (l *Ledger) Transfer(caller string, req Request) (Result, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r := Result{RequestID: req.RequestID, Caller: caller, Amount: req.Amount, Currency: req.Currency, Snapshot: l.snapshot()}
	if caller == "" || !requestID.MatchString(req.RequestID) {
		r.Error = "invalid_request_id"
		return r, 400
	}
	k := key{caller, req.RequestID}
	if old, ok := l.history[k]; ok {
		if old.request != req {
			r.Error = "idempotency_conflict"
			return r, 409
		}
		r = clone(old.result)
		r.Replay = true
		return r, old.status
	}
	if len(l.history) >= l.capacity {
		r.Error = "history_capacity_exhausted"
		return r, 503
	}
	status := 200
	switch {
	case req.From != "demo-a" || req.To != "demo-b" || req.Amount != 100 || req.Currency != "USD":
		r.Error = "invalid_fixed_transfer"
		status = 400
	case l.balances[req.From] < req.Amount:
		r.Error = "insufficient_funds"
		status = 409
	case l.balances[req.To] > math.MaxInt64-req.Amount || l.version == math.MaxUint64:
		r.Error = "overflow"
		status = 409
	default:
		l.balances[req.From] -= req.Amount
		l.balances[req.To] += req.Amount
		l.version++
		sum := sha256.Sum256([]byte("poc-spire-transfer\x00" + caller + "\x00" + req.RequestID))
		r.TransferID = hex.EncodeToString(sum[:])
		r.Snapshot = l.snapshot()
	}
	l.history[k] = record{req, clone(r), status}
	return r, status
}
