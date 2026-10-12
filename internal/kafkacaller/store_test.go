package kafkacaller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"poc-spire/internal/kafkacmd"
)

func testStore(t *testing.T) (*Store, string, kafkacmd.Policy) {
	t.Helper()
	policy, _ := kafkacmd.NewPolicy("0123456789abcdef0123456789abcdef")
	dir := filepath.Join(t.TempDir(), "caller")
	if err := Initialize(dir, policy, "unidentified"); err != nil {
		t.Fatal(err)
	}
	store, err := Open(dir, policy, "unidentified")
	if err != nil {
		t.Fatal(err)
	}
	return store, dir, policy
}
func TestPublicationReservationSurvivesAckLossAndCallerRestart(t *testing.T) {
	store, dir, policy := testStore(t)
	now := time.Unix(1791780000, 0)
	instance := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	published := 0
	sender := &Sender{Store: store, Clock: func() time.Time { return now }, NoIdentity: func(context.Context) error { return nil }, Publish: func(_ context.Context, r kafkacmd.Record) error {
		published++
		reopened, err := Open(dir, policy, "unidentified")
		if err != nil {
			t.Fatal(err)
		}
		metadata, code, err := reopened.Lookup("one")
		if err != nil || code != 200 || len(metadata.Variants) != 1 || metadata.Variants[0].Digest != kafkacmd.Digest(r) || metadata.Variants[0].Publication != "publish_unknown" {
			t.Fatal("publish ran before durable reservation", metadata, err)
		}
		return errors.New("ack_lost")
	}}
	first, code, err := sender.Send(context.Background(), "one", "kafka-unidentified", instance)
	if err != nil || code != 200 || first.Publication != "publish_unknown" || published != 1 {
		t.Fatal(first, code, err)
	}
	now = now.Add(20 * time.Second)
	reopened, err := Open(dir, policy, "unidentified")
	if err != nil {
		t.Fatal(err)
	}
	recovered, code, err := reopened.Lookup("one")
	if err != nil || code != 200 || !reflect.DeepEqual(first, recovered) {
		t.Fatal("restart changed descriptor")
	}
	sender.Store = reopened
	sender.Publish = func(_ context.Context, r kafkacmd.Record) error {
		published++
		if kafkacmd.Digest(r) != first.Variants[0].Digest {
			t.Fatal("retry changed exact unsigned bytes")
		}
		return nil
	}
	second, code, err := sender.Send(context.Background(), "one", "kafka-unidentified", instance)
	if err != nil || code != 200 || second.CreatedAt != first.CreatedAt || second.Deadline != first.Deadline || second.MoneyInstanceID != first.MoneyInstanceID || len(second.Variants) != 1 || second.Publication != "queued" {
		t.Fatal(second, err)
	}
	now = time.Unix(first.Deadline, 0)
	if _, code, err := sender.Send(context.Background(), "one", "kafka-unidentified", instance); err == nil || code != 409 || published != 2 {
		t.Fatal("expired descriptor renewed")
	}
	if after, code, err := reopened.Lookup("one"); err != nil || code != 200 || after.Deadline != first.Deadline {
		t.Fatal("expired metadata lost")
	}
}
func TestInitializedStoreNeverRepairsLostManifestIndexOrKnownDescriptor(t *testing.T) {
	for _, mode := range []string{"manifest-missing", "state-missing", "state-corrupt", "entry-missing", "stream", "permissions", "lock-missing"} {
		t.Run(mode, func(t *testing.T) {
			store, dir, policy := testStore(t)
			store.allocate("known", "kafka-unidentified", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", time.Now())
			switch mode {
			case "manifest-missing":
				os.Remove(filepath.Join(dir, "manifest.json"))
			case "state-missing":
				os.Remove(filepath.Join(dir, "state.json"))
			case "state-corrupt":
				os.WriteFile(filepath.Join(dir, "state.json"), []byte("broken"), 0600)
			case "entry-missing":
				current, err := store.load()
				if err != nil {
					t.Fatal(err)
				}
				delete(current.Entries, "known")
				raw, _ := marshalState(current)
				os.WriteFile(filepath.Join(dir, "state.json"), raw, 0600)
			case "stream":
				policy, _ = kafkacmd.NewPolicy("11111111111111111111111111111111")
			case "permissions":
				os.Chmod(filepath.Join(dir, "state.json"), 0644)
			case "lock-missing":
				os.Remove(filepath.Join(dir, "state.lock"))
			}
			if _, err := Open(dir, policy, "unidentified"); err == nil {
				t.Fatal("lost state recreated or accepted")
			}
			if _, _, err := store.Lookup("known"); mode != "stream" && err == nil {
				t.Fatal("corrupt runtime state remained usable")
			}
			if err := Initialize(dir, policy, "unidentified"); err == nil {
				t.Fatal("initialization repaired existing directory")
			}
		})
	}
}
func TestDescriptorCapacityConflictAndNoIdentityFailure(t *testing.T) {
	store, _, _ := testStore(t)
	now := time.Unix(1791780000, 0)
	instance := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for i := 0; i < 64; i++ {
		if _, code, err := store.allocate(fmt.Sprintf("id-%d", i), "kafka-unidentified", instance, now); err != nil || code != 200 {
			t.Fatal(i, err)
		}
	}
	if _, code, err := store.allocate("new", "kafka-unidentified", instance, now); err == nil || code != 503 {
		t.Fatal("capacity evicted retry state")
	}
	if _, code, err := store.allocate("id-0", "kafka-unidentified", instance, now.Add(time.Second)); err != nil || code != 200 {
		t.Fatal("existing descriptor blocked at capacity")
	}
	if _, code, err := store.allocate("id-0", "kafka-unidentified", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", now); err == nil || code != 409 {
		t.Fatal("instance conflict refreshed state")
	}
	published := false
	sender := &Sender{Store: store, Clock: func() time.Time { return now }, NoIdentity: func(context.Context) error { return errors.New("unexpected_identity_issued") }, Publish: func(context.Context, kafkacmd.Record) error { published = true; return nil }}
	if _, _, err := sender.Send(context.Background(), "id-0", "kafka-unidentified", instance); err == nil || published {
		t.Fatal("unexpected issuance published unsigned fallback")
	}
	// The index explicitly retains an unseen-ID distinction only while initialized.
	if m, code, err := store.Lookup("unseen"); err != nil || code != 404 || !m.Initialized || m.Exists {
		t.Fatal(m, code, err)
	}
}
func TestCallerMetadataNeverExportsRecordBytes(t *testing.T) {
	store, _, _ := testStore(t)
	sender := &Sender{Store: store, NoIdentity: func(context.Context) error { return nil }, Publish: func(context.Context, kafkacmd.Record) error { return nil }}
	m, _, err := sender.Send(context.Background(), "one", "kafka-unidentified", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(m)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	for _, name := range []string{"record", "value", "key", "x5c", "private_key"} {
		if _, ok := fields[name]; ok {
			t.Fatal("private state exported")
		}
	}
}
