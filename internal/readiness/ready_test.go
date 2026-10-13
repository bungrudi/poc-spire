package readiness

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeState(t *testing.T, path string, s State) {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLivenessOrStaleMarkerCannotImplyReadiness(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ready.json")
	owner, err := process(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []State{
		{Ready: false, At: time.Now().Unix(), Process: owner},
		{Ready: true, At: time.Now().Unix() - 10, Process: owner},
		{Ready: true, At: time.Now().Unix() + 10, Process: owner},
		{Ready: true, At: time.Now().Unix()},
	} {
		writeState(t, p, s)
		if Check(p) {
			t.Fatal("nonready/stale/future/unbound accepted", s)
		}
	}
	s := State{Ready: true, At: time.Now().Unix(), Process: owner}
	writeState(t, p, s)
	if !Check(p) {
		t.Fatal("current native marker rejected")
	}
	s.Process.Start++
	writeState(t, p, s)
	if Check(p) {
		t.Fatal("previous process incarnation accepted")
	}
	s.Process = owner
	s.Process.Namespace = "pid:[0]"
	writeState(t, p, s)
	if Check(p) {
		t.Fatal("previous container namespace accepted")
	}
	if err = os.WriteFile(p, []byte(strings.Repeat("x", markerLimit+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if Check(p) {
		t.Fatal("oversized marker accepted")
	}
}

func TestReadinessChild(t *testing.T) {
	if os.Getenv("POC_READY_CHILD") != "1" {
		return
	}
	p := os.Getenv("POC_READY_PATH")
	release := os.Getenv("POC_READY_RELEASE")
	if release != "" {
		for {
			if _, err := os.Stat(release); err == nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	Start(context.Background(), p, func(context.Context) bool { return true })
	select {}
}

func TestSIGKILLRestartRetainedMarkerDoesNotAuthorizeStartup(t *testing.T) {
	dir := t.TempDir()
	p, release := filepath.Join(dir, "ready.json"), filepath.Join(dir, "release")
	child := func(block bool) *exec.Cmd {
		c := exec.Command(os.Args[0], "-test.run=^TestReadinessChild$")
		c.Env = append(os.Environ(), "POC_READY_CHILD=1", "POC_READY_PATH="+p)
		if block {
			c.Env = append(c.Env, "POC_READY_RELEASE="+release)
		}
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Process.Kill(); c.Wait() })
		return c
	}
	waitReady := func() {
		deadline := time.Now().Add(2 * time.Second)
		for !Check(p) {
			if time.Now().After(deadline) {
				t.Fatal("child never ready")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	first := child(false)
	waitReady()
	before, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	first.Wait()
	child(true)
	if _, err = os.Stat(p); err != nil {
		t.Fatal("retained marker missing")
	}
	if Check(p) {
		t.Fatal("SIGKILL marker authorized replacement before native startup")
	}
	after, _ := os.ReadFile(p)
	if string(after) != string(before) {
		t.Fatal("restart regression did not preserve old emptyDir marker")
	}
	if err = os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	waitReady()
}
