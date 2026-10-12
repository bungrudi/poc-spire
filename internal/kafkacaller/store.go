// Package kafkacaller retains bounded per-run command recovery data. Initialization
// belongs to the runner; application startup never recreates lost state.
package kafkacaller

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"poc-spire/internal/kafkacmd"
)

const MaxState = 16 << 20

type Manifest struct {
	Version  int    `json:"version"`
	StreamID string `json:"stream_id"`
	Nonce    string `json:"nonce"`
	Role     string `json:"role"`
}
type Variant struct {
	Record      kafkacmd.Record `json:"record"`
	Digest      string          `json:"digest"`
	ExpiresAt   int64           `json:"expires_at"`
	Publication string          `json:"publication"`
}
type Descriptor struct {
	RequestID       string           `json:"request_id"`
	Scenario        string           `json:"scenario"`
	MoneyInstanceID string           `json:"money_instance_id"`
	Context         kafkacmd.Context `json:"context"`
	Variants        []Variant        `json:"variants"`
}
type state struct {
	Version  int                   `json:"version"`
	StreamID string                `json:"stream_id"`
	Nonce    string                `json:"nonce"`
	Index    []string              `json:"index"`
	Entries  map[string]Descriptor `json:"entries"`
}
type diskState struct {
	Payload  json.RawMessage `json:"payload"`
	Checksum string          `json:"checksum"`
}
type VariantMetadata struct {
	Digest      string `json:"digest"`
	ExpiresAt   int64  `json:"expires_at"`
	Publication string `json:"publication"`
}
type Metadata struct {
	Initialized     bool              `json:"initialized"`
	Exists          bool              `json:"exists"`
	RequestID       string            `json:"request_id"`
	Scenario        string            `json:"scenario,omitempty"`
	StreamID        string            `json:"stream_id"`
	MoneyInstanceID string            `json:"money_instance_id,omitempty"`
	CreatedAt       int64             `json:"created_at,omitempty"`
	Deadline        int64             `json:"deadline,omitempty"`
	Publication     string            `json:"publication"`
	Variants        []VariantMetadata `json:"variants"`
}
type Store struct {
	dir    string
	policy kafkacmd.Policy
	role   string
}

func allowed(role, scenario string) bool {
	return role == "trusted" && (scenario == "kafka-trusted" || scenario == "kafka-tampered") || role == "unidentified" && scenario == "kafka-unidentified"
}
func checksum(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func marshalState(s state) ([]byte, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return json.Marshal(diskState{raw, checksum(raw)})
}

// Initialize fails if this caller's directory already exists. It is not a reset
// or repair operation and is never invoked by a caller process.
func Initialize(dir string, policy kafkacmd.Policy, role string) error {
	if !policy.Valid() || (role != "trusted" && role != "unidentified") || !filepath.IsAbs(dir) {
		return errors.New("invalid_store_initialization")
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	nonce, err := kafkacmd.Nonce()
	if err != nil {
		return err
	}
	manifest, _ := json.Marshal(Manifest{1, policy.StreamID, nonce, role})
	raw, err := marshalState(state{1, policy.StreamID, nonce, []string{}, map[string]Descriptor{}})
	if err != nil {
		return err
	}
	for _, entry := range []struct {
		name  string
		value []byte
	}{{"manifest.json", manifest}, {"state.json", raw}, {"state.lock", nil}} {
		file, err := os.OpenFile(filepath.Join(dir, entry.name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, err = file.Write(entry.value)
		if err == nil {
			err = file.Sync()
		}
		file.Close()
		if err != nil {
			return err
		}
	}
	return syncDirectory(dir)
}
func syncDirectory(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

// Configure creates no files. Every control call still requires intact initialized
// state; failure disables Kafka controls while allowing existing Sync controls.
func Configure(dir string, policy kafkacmd.Policy, role string) (*Store, error) {
	if !policy.Valid() || !filepath.IsAbs(dir) || (role != "trusted" && role != "unidentified") {
		return nil, errors.New("invalid_store_configuration")
	}
	return &Store{dir, policy, role}, nil
}

func Open(dir string, policy kafkacmd.Policy, role string) (*Store, error) {
	store := &Store{dir, policy, role}
	if !policy.Valid() || !filepath.IsAbs(dir) {
		return nil, errors.New("invalid_store_configuration")
	}
	lock, err := store.lock()
	if err != nil {
		return nil, err
	}
	defer unlock(lock)
	if _, err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}
func privateFile(path string, max int) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > int64(max) {
		return nil, errors.New("caller_state_unavailable")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("caller_state_owner")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, int64(max)+1))
	if err != nil || len(raw) > max {
		return nil, errors.New("caller_state_bounds")
	}
	return raw, nil
}
func (s *Store) lock() (*os.File, error) {
	info, err := os.Lstat(s.dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, errors.New("caller_state_unavailable")
	}
	if _, err := privateFile(filepath.Join(s.dir, "state.lock"), 0); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(s.dir, "state.lock"), os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("caller_state_busy")
	}
	return file, nil
}
func unlock(file *os.File) { unix.Flock(int(file.Fd()), unix.LOCK_UN); file.Close() }
func strictDecode(raw []byte, out any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("trailing_state")
	}
	return nil
}
func (s *Store) load() (state, error) {
	raw, err := privateFile(filepath.Join(s.dir, "manifest.json"), 1024)
	if err != nil {
		return state{}, err
	}
	var manifest Manifest
	if strictDecode(raw, &manifest) != nil || manifest.Version != 1 || manifest.StreamID != s.policy.StreamID || manifest.Role != s.role || !kafkacmd.Hex32.MatchString(manifest.Nonce) {
		return state{}, errors.New("caller_manifest_mismatch")
	}
	raw, err = privateFile(filepath.Join(s.dir, "state.json"), MaxState)
	if err != nil {
		return state{}, err
	}
	var disk diskState
	if strictDecode(raw, &disk) != nil || checksum(disk.Payload) != disk.Checksum {
		return state{}, errors.New("caller_state_corrupt")
	}
	var current state
	if strictDecode(disk.Payload, &current) != nil || current.Version != 1 || current.StreamID != s.policy.StreamID || current.Nonce != manifest.Nonce || current.Entries == nil || current.Index == nil || len(current.Entries) != len(current.Index) || len(current.Index) > 64 {
		return state{}, errors.New("caller_index_corrupt")
	}
	seen := map[string]bool{}
	for _, id := range current.Index {
		descriptor, exists := current.Entries[id]
		if !exists || seen[id] || !kafkacmd.RequestID.MatchString(id) || descriptor.RequestID != id || !allowed(s.role, descriptor.Scenario) || !kafkacmd.Hex32.MatchString(descriptor.MoneyInstanceID) || len(descriptor.Variants) > 4 {
			return state{}, errors.New("caller_descriptor_corrupt")
		}
		seen[id] = true
		c := descriptor.Context
		expected := s.policy.Context(time.Unix(c.CreatedAt, 0))
		expected.Deadline = c.Deadline
		if c != expected || c.CreatedAt < 0 || c.Deadline > 4102444800 || c.Deadline <= c.CreatedAt || c.Deadline-c.CreatedAt > 120 {
			return state{}, errors.New("caller_descriptor_corrupt")
		}
		digests := map[string]bool{}
		for _, v := range descriptor.Variants {
			if !kafkacmd.Hex64.MatchString(v.Digest) || digests[v.Digest] || kafkacmd.Digest(v.Record) != v.Digest || v.Record.Topic != s.policy.Topic || string(v.Record.Key) != id || len(v.Record.Value) > kafkacmd.MaxValue || len(v.Record.Headers) != 0 || v.ExpiresAt <= c.CreatedAt || v.ExpiresAt > c.Deadline || (v.Publication != "publish_unknown" && v.Publication != "queued") {
				return state{}, errors.New("caller_variant_corrupt")
			}
			digests[v.Digest] = true
		}
	}
	return current, nil
}
func (s *Store) save(current state) error {
	raw, err := marshalState(current)
	if err != nil || len(raw) > MaxState {
		return errors.New("caller_state_bounds")
	}
	file, err := os.CreateTemp(s.dir, ".state-")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	file.Close()
	if err != nil {
		return err
	}
	if err = os.Rename(name, filepath.Join(s.dir, "state.json")); err != nil {
		return err
	}
	return syncDirectory(s.dir)
}
func metadata(descriptor Descriptor, stream string) Metadata {
	m := Metadata{Initialized: true, Exists: true, RequestID: descriptor.RequestID, Scenario: descriptor.Scenario, StreamID: stream, MoneyInstanceID: descriptor.MoneyInstanceID, CreatedAt: descriptor.Context.CreatedAt, Deadline: descriptor.Context.Deadline, Publication: "not_submitted", Variants: []VariantMetadata{}}
	for _, v := range descriptor.Variants {
		m.Variants = append(m.Variants, VariantMetadata{v.Digest, v.ExpiresAt, v.Publication})
		m.Publication = v.Publication
	}
	return m
}
func (s *Store) Lookup(id string) (Metadata, int, error) {
	if !kafkacmd.RequestID.MatchString(id) {
		return Metadata{}, 400, errors.New("invalid_request_id")
	}
	lock, err := s.lock()
	if err != nil {
		return Metadata{}, 503, err
	}
	defer unlock(lock)
	current, err := s.load()
	if err != nil {
		return Metadata{}, 503, err
	}
	descriptor, exists := current.Entries[id]
	if !exists {
		return Metadata{Initialized: true, RequestID: id, StreamID: s.policy.StreamID, Publication: "not_submitted", Variants: []VariantMetadata{}}, 404, nil
	}
	return metadata(descriptor, s.policy.StreamID), 200, nil
}
func (s *Store) mutate(id string, fn func(*Descriptor) error) (Metadata, error) {
	lock, err := s.lock()
	if err != nil {
		return Metadata{}, err
	}
	defer unlock(lock)
	current, err := s.load()
	if err != nil {
		return Metadata{}, err
	}
	descriptor, ok := current.Entries[id]
	if !ok {
		return Metadata{}, errors.New("expected_descriptor_missing")
	}
	if err := fn(&descriptor); err != nil {
		return Metadata{}, err
	}
	current.Entries[id] = descriptor
	if err := s.save(current); err != nil {
		return Metadata{}, err
	}
	return metadata(descriptor, s.policy.StreamID), nil
}
func (s *Store) allocate(id, scenario, instance string, now time.Time) (Descriptor, int, error) {
	if !kafkacmd.RequestID.MatchString(id) || !allowed(s.role, scenario) || !kafkacmd.Hex32.MatchString(instance) {
		return Descriptor{}, 400, errors.New("invalid_fixed_command")
	}
	lock, err := s.lock()
	if err != nil {
		return Descriptor{}, 503, err
	}
	defer unlock(lock)
	current, err := s.load()
	if err != nil {
		return Descriptor{}, 503, err
	}
	if old, ok := current.Entries[id]; ok {
		if old.Scenario != scenario || old.MoneyInstanceID != instance {
			return old, 409, errors.New("descriptor_conflict")
		}
		if now.Unix() >= old.Context.Deadline {
			return old, 409, errors.New("descriptor_expired")
		}
		return old, 200, nil
	}
	if len(current.Index) >= 64 {
		return Descriptor{}, 503, errors.New("descriptor_capacity_exhausted")
	}
	descriptor := Descriptor{RequestID: id, Scenario: scenario, MoneyInstanceID: instance, Context: s.policy.Context(now), Variants: []Variant{}}
	current.Index = append(current.Index, id)
	current.Entries[id] = descriptor
	if err := s.save(current); err != nil {
		return Descriptor{}, 503, fmt.Errorf("descriptor_not_reserved: %w", err)
	}
	return descriptor, 200, nil
}
