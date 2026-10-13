package callerstatecheck

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/kafkacmd"
	"testing"
)

func TestInitializedCallerCorruptionLossAndFreshNonce(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "private")
	stream := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	policy, _ := kafkacmd.NewPolicy(stream)
	if err := kafkacaller.Initialize(dir, policy, "trusted"); err != nil {
		t.Fatal(err)
	}
	first := Check("caller-trusted", dir, "run", "pod", stream)
	if !first.Valid {
		t.Fatal("valid store rejected")
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if Check("caller-trusted", dir, "run", "pod", stream).Valid {
		t.Fatal("corrupt initialized state accepted")
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if Check("caller-trusted", dir, "run", "pod", stream).Valid {
		t.Fatal("missing state accepted")
	}
	if err := kafkacaller.Initialize(dir, policy, "trusted"); err != nil {
		t.Fatal(err)
	}
	next := Check("caller-trusted", dir, "run", "pod", stream)
	if !next.Valid || next.Incarnation == first.Incarnation {
		t.Fatal("same-run store replacement not distinguishable")
	}
}

func TestHeldCallerPublicationLockIsUnavailableNotDestructive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	stream := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	policy, _ := kafkacmd.NewPolicy(stream)
	if err := kafkacaller.Initialize(dir, policy, "trusted"); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(dir, "state.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	result := Check("caller-trusted", dir, "run", "pod", stream)
	if result.Valid || !result.Unavailable {
		t.Fatal("ordinary publication lock classified as destructive", result)
	}
	unix.Flock(int(file.Fd()), unix.LOCK_UN)
	result = Check("caller-trusted", dir, "run", "pod", stream)
	if !result.Valid || result.Unavailable {
		t.Fatal("released valid state did not recover", result)
	}
}
