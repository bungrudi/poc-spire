package statecheck

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"

	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/kafkacmd"
)

func TestInitializedCallerCorruptionLossAndFreshNonce(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "private")
	stream := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	policy, _ := kafkacmd.NewPolicy(stream)
	if err := kafkacaller.Initialize(dir, policy, "trusted"); err != nil {
		t.Fatal(err)
	}
	first := Check("caller-trusted", dir, "run", "pod", stream, "", false)
	if !first.Valid {
		t.Fatal("valid store rejected")
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if Check("caller-trusted", dir, "run", "pod", stream, "", false).Valid {
		t.Fatal("corrupt initialized state accepted")
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if Check("caller-trusted", dir, "run", "pod", stream, "", false).Valid {
		t.Fatal("missing state accepted")
	}
	if err := kafkacaller.Initialize(dir, policy, "trusted"); err != nil {
		t.Fatal(err)
	}
	next := Check("caller-trusted", dir, "run", "pod", stream, "", false)
	if !next.Valid || next.Incarnation == first.Incarnation {
		t.Fatal("same-run store replacement not distinguishable")
	}
}

func TestBrokerMarkerAndMetadataBoundToRun(t *testing.T) {
	dir := t.TempDir()
	stream := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	put := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put("owned-initialized", "run\npod")
	put("meta.properties", "cluster.id=cluster\nnode.id=1\nversion=1\n")
	first := Check("kafka", dir, "run", "pod", stream, "cluster", false)
	if !first.Valid {
		t.Fatal("initialized broker rejected")
	}
	if Check("kafka", dir, "other", "pod", stream, "cluster", false).Valid || Check("kafka", dir, "run", "pod", stream, "foreign", false).Valid {
		t.Fatal("foreign broker incarnation accepted")
	}
	if Check("kafka", dir, "run", "pod", stream, "cluster", true).Valid {
		t.Fatal("lost broker topics accepted")
	}
	for _, name := range []string{"__cluster_metadata-0", "poc.spire.commands." + stream + "-0"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if !Check("kafka", dir, "run", "pod", stream, "cluster", true).Valid {
		t.Fatal("complete broker rejected")
	}
	put("meta.properties", "cluster.id=cluster\nnode.id=1\nversion=1\ncluster.id=foreign\n")
	if Check("kafka", dir, "run", "pod", stream, "cluster", false).Valid {
		t.Fatal("ambiguous broker metadata accepted")
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
	result := Check("caller-trusted", dir, "run", "pod", stream, "", false)
	if result.Valid || !result.Unavailable {
		t.Fatal("ordinary publication lock classified as destructive", result)
	}
	unix.Flock(int(file.Fd()), unix.LOCK_UN)
	result = Check("caller-trusted", dir, "run", "pod", stream, "", false)
	if !result.Valid || result.Unavailable {
		t.Fatal("released valid state did not recover", result)
	}
}
