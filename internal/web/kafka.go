package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/ledger"
)

type KafkaDependencies struct {
	Metadata func(context.Context) (ledger.KafkaMetadata, error)
	Receipt  func(context.Context, string) (ledger.Receipt, ledger.KafkaMetadata, int, error)
	Lookup   func(context.Context, string, string) (kafkacaller.Metadata, int, error)
	Publish  func(context.Context, string, kafkacaller.PublishRequest) (kafkacaller.Metadata, int, error)
}
type OutcomeQuery struct {
	Digest     string          `json:"digest"`
	HTTPStatus int             `json:"query_http_status"`
	Receipt    *ledger.Receipt `json:"receipt,omitempty"`
}
type KafkaResult struct {
	Stage      string                `json:"stage"`
	Outcome    string                `json:"outcome"`
	Provenance string                `json:"provenance"`
	Caller     *kafkacaller.Metadata `json:"caller_diagnostics,omitempty"`
	Queries    []OutcomeQuery        `json:"money_queries"`
}

func kafkaKind(scenario string) (string, bool) {
	switch scenario {
	case "kafka-trusted", "kafka-tampered":
		return "trusted", true
	case "kafka-unidentified":
		return "unidentified", true
	}
	return "", false
}
func (a *App) createKafka(w http.ResponseWriter, r *http.Request, p Principal) {
	if a.config.KafkaStream == "" {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid bounded Kafka attempt", 400)
		return
	}
	obj, err := kafkacmd.StrictJSON(raw)
	valid := err == nil && len(obj) == 3
	for _, name := range []string{"scenario", "browser_key", "stream_id"} {
		if _, ok := obj[name].(string); !ok {
			valid = false
		}
	}
	var in struct {
		Scenario string `json:"scenario"`
		Key      string `json:"browser_key"`
		StreamID string `json:"stream_id"`
	}
	if !valid || json.Unmarshal(raw, &in) != nil || !browserKey.MatchString(in.Key) || in.StreamID != a.config.KafkaStream {
		http.Error(w, "invalid scenario or stale demo stream", 400)
		return
	}
	kind, ok := kafkaKind(in.Scenario)
	if !ok {
		http.Error(w, "unsupported fixed scenario", 400)
		return
	}
	a.dispatch(w, r, p, RequestID(p, in.Key), kind, in.Scenario, in.StreamID)
}
func (a *App) render(w http.ResponseWriter, r *http.Request, s session) {
	values := map[string]any{"Local": a.config.Local, "Logged": true, "Role": s.Principal.Role, "Kafka": a.config.KafkaStream != "", "KafkaStream": a.config.KafkaStream, "KafkaReady": false}
	if a.deps.Kafka != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		metadata, err := a.deps.Kafka.Metadata(ctx)
		cancel()
		values["KafkaReady"] = err == nil && metadata.StreamID == a.config.KafkaStream && kafkacmd.Hex32.MatchString(metadata.InstanceID) && metadata.ConsumerReady
	}
	page.Execute(w, values)
}

// settle accepts only authenticated, instance/digest-correlated terminal evidence.
// One applied variant resolves success; rejection needs every potential variant.
func settle(id, scenario, instance string, m kafkacaller.Metadata, queries []OutcomeQuery, now int64) (string, error) {
	if len(m.Variants) == 0 {
		return "", nil
	}
	receipts := map[string]*ledger.Receipt{}
	for _, q := range queries {
		if q.HTTPStatus != 200 || q.Receipt == nil {
			continue
		}
		r := q.Receipt
		if now >= r.RetainUntil {
			continue
		}
		if r.InstanceID != instance || r.Digest != q.Digest {
			return "", errors.New("uncorrelated_money_evidence")
		}
		receipts[q.Digest] = r
	}
	allRejected := true
	for _, v := range m.Variants {
		r, ok := receipts[v.Digest]
		if !ok {
			allRejected = false
			continue
		}
		switch r.Disposition {
		case "applied", "replayed":
			result := r.BusinessResult
			sum := sha256.Sum256([]byte("poc-spire-transfer\x00" + identity.Trusted.String() + "\x00" + id))
			if scenario != "kafka-trusted" || r.VerifiedProducer != identity.Trusted.String() || r.RequestID != id || r.BusinessCode != 200 || result == nil || result.RequestID != id || result.Caller != identity.Trusted.String() || result.Amount != 100 || result.Currency != "USD" || result.Error != "" || result.TransferID != hex.EncodeToString(sum[:]) || result.Version == 0 || len(result.Balances) != 2 || result.Balances["demo-a"] < 0 || result.Balances["demo-b"] < 0 || result.Balances["demo-a"]+result.Balances["demo-b"] != 200000 {
				return "", errors.New("unexpected_money_business_evidence")
			}
			return "transfer_succeeded", nil
		case "rejected":
			if r.BusinessResult != nil || r.BusinessCode != 0 {
				return "", errors.New("invalid_rejection_evidence")
			}
			if scenario != "kafka-trusted" && (r.VerifiedProducer != "" || r.RequestID != "") {
				return "", errors.New("unexpected_attributed_negative_probe")
			}
		case "business_rejected", "conflicted":
			if r.VerifiedProducer != identity.Trusted.String() || r.RequestID != id || r.BusinessCode < 400 {
				return "", errors.New("invalid_business_rejection")
			}
		default:
			return "", errors.New("unknown_money_disposition")
		}
	}
	if allRejected {
		return "money_rejected", nil
	}
	return "", nil
}
func (a *App) kafkaStage(attempt *Attempt, result KafkaResult) {
	a.mu.Lock()
	defer a.mu.Unlock()
	copy := result
	copy.Queries = append([]OutcomeQuery{}, result.Queries...)
	if result.Caller != nil {
		m := *result.Caller
		m.Variants = append([]kafkacaller.VariantMetadata{}, result.Caller.Variants...)
		copy.Caller = &m
	}
	attempt.Kafka = &copy
	attempt.Updated = a.now()
}
func (a *App) kafkaWorker(attempt *Attempt) {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	result := KafkaResult{Stage: "preparing", Outcome: "unknown_commit", Provenance: "caller diagnostics; terminal effects require authenticated exact-Money receipts", Queries: []OutcomeQuery{}}
	defer func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		attempt.Kafka = &result
		attempt.Updated = a.now()
		attempt.Status = "retryable"
		if result.Outcome == "transfer_succeeded" || result.Outcome == "money_rejected" {
			attempt.Status = "done"
		}
		a.active--
	}()
	if a.deps.RunValid != nil && !a.deps.RunValid() {
		result.Stage = "run_guard"
		return
	}
	a.kafkaStage(attempt, result)
	deps := a.deps.Kafka
	readMeta := func() (ledger.KafkaMetadata, error) {
		query, stop := context.WithTimeout(ctx, 2*time.Second)
		defer stop()
		m, err := deps.Metadata(query)
		if err != nil || m.StreamID != attempt.StreamID || !kafkacmd.Hex32.MatchString(m.InstanceID) {
			return m, errors.New("money_metadata_unavailable_or_reset")
		}
		return m, nil
	}
	lookup := func() (kafkacaller.Metadata, int, error) {
		if a.deps.RunValid != nil && !a.deps.RunValid() {
			return kafkacaller.Metadata{}, 503, errors.New("run_invalid")
		}
		query, stop := context.WithTimeout(ctx, 2*time.Second)
		defer stop()
		return deps.Lookup(query, attempt.Scenario, attempt.ID)
	}
	money, err := readMeta()
	if err != nil {
		result.Stage = "money_unavailable"
		return
	}
	metadata, code, err := lookup()
	if err != nil || !metadata.Initialized || metadata.RequestID != attempt.ID || metadata.StreamID != attempt.StreamID || len(metadata.Variants) > 4 || (code == 200) != metadata.Exists || (code != 200 && code != 404) {
		result.Stage = "caller_state_unknown"
		return
	}
	result.Caller = &metadata
	if metadata.Exists && (metadata.Scenario != attempt.Scenario || metadata.MoneyInstanceID != money.InstanceID) {
		result.Stage = "money_reset_or_descriptor_conflict"
		return
	}
	queried := map[string]time.Time{}
	evidence := map[string]OutcomeQuery{}
	// Each retained variant is queried before any first publication or retry.
	queryAll := func() error {
		var queryFailure error
		for _, v := range metadata.Variants {
			if last, ok := queried[v.Digest]; ok && time.Since(last) < time.Second {
				continue
			}
			queried[v.Digest] = time.Now()
			query, stop := context.WithTimeout(ctx, 2*time.Second)
			r, m, status, err := deps.Receipt(query, v.Digest)
			stop()
			observation := OutcomeQuery{Digest: v.Digest, HTTPStatus: status}
			if err != nil {
				evidence[v.Digest] = observation
				queryFailure = errors.New("money_receipt_unknown")
				continue
			}
			if m.InstanceID != money.InstanceID || (status == 404 && m.StreamID != attempt.StreamID) {
				return errors.New("money_instance_reset")
			}
			if status == 200 {
				if r.InstanceID != money.InstanceID || r.Digest != v.Digest {
					evidence[v.Digest] = observation
					queryFailure = errors.New("uncorrelated_money_receipt")
					continue
				}
				copy := r
				observation.Receipt = &copy
			} else if status != 404 {
				queryFailure = errors.New("money_receipt_unknown")
			}
			evidence[v.Digest] = observation
		}
		result.Queries = []OutcomeQuery{}
		for _, v := range metadata.Variants {
			if q, ok := evidence[v.Digest]; ok {
				result.Queries = append(result.Queries, q)
			}
		}
		outcome, err := settle(attempt.ID, attempt.Scenario, money.InstanceID, metadata, result.Queries, a.now().Unix())
		if err != nil {
			return err
		}
		if outcome != "" {
			result.Outcome = outcome
			result.Stage = "money_verified"
			if outcome == "transfer_succeeded" {
				result.Provenance = "authenticated exact-Money receipt for the original producer and logical transfer"
			} else if attempt.Scenario != "kafka-trusted" {
				result.Provenance = "authenticated Money rejection for exact record bytes; unattributed invalid record"
			} else {
				result.Provenance = "authenticated Money no-effect receipts for every potentially submitted record"
			}
			return nil
		}
		return queryFailure
	}
	if err := queryAll(); err != nil {
		result.Stage = "evidence_unknown_or_reset"
		return
	}
	if result.Outcome != "unknown_commit" {
		return
	}
	// Publication is allowed only after reading all existing evidence. An expired
	// original deadline, changed Money nonce, or unavailable store never creates one.
	if !metadata.Exists || a.now().Unix() < metadata.Deadline {
		if money.ConsumerReady {
			if a.deps.RunValid != nil && !a.deps.RunValid() {
				result.Stage = "run_guard"
				return
			}
			publish, stop := context.WithTimeout(ctx, 10*time.Second)
			m, status, err := deps.Publish(publish, attempt.Scenario, kafkacaller.PublishRequest{RequestID: attempt.ID, Scenario: attempt.Scenario, MoneyInstanceID: money.InstanceID})
			stop()
			if err == nil && status == 200 {
				metadata = m
			} else {
				metadata, code, err = lookup()
				if err != nil || code != 200 {
					result.Stage = "caller_publication_unknown"
					return
				}
			}
			if !metadata.Initialized || !metadata.Exists || metadata.RequestID != attempt.ID || metadata.Scenario != attempt.Scenario || metadata.StreamID != attempt.StreamID || metadata.MoneyInstanceID != money.InstanceID || len(metadata.Variants) > 4 {
				result.Stage = "uncorrelated_caller_publication"
				return
			}
			result.Caller = &metadata
			result.Stage = metadata.Publication
		}
	}
	for ctx.Err() == nil {
		if err := queryAll(); err != nil {
			result.Stage = "evidence_unknown_or_reset"
			return
		}
		if result.Outcome != "unknown_commit" {
			return
		}
		result.Stage = "awaiting_money"
		a.kafkaStage(attempt, result)
		select {
		case <-ctx.Done():
			result.Stage = "unknown"
			return
		case <-time.After(time.Second):
		}
		latest, err := readMeta()
		if err != nil || latest.InstanceID != money.InstanceID {
			result.Stage = "money_reset_or_unavailable"
			return
		}
	}
	result.Stage = "unknown"
}
