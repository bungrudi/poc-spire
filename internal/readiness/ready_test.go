package readiness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLivenessOrStaleMarkerCannotImplyReadiness(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ready.json")
	for _, s := range []State{{false, time.Now().Unix()}, {true, time.Now().Unix() - 10}, {true, time.Now().Unix() + 10}} {
		b, _ := json.Marshal(s)
		os.WriteFile(p, b, 0600)
		if Check(p) {
			t.Fatal("nonready/stale/future accepted", s)
		}
	}
	b, _ := json.Marshal(State{true, time.Now().Unix()})
	os.WriteFile(p, b, 0600)
	if !Check(p) {
		t.Fatal("current native marker rejected")
	}
}
