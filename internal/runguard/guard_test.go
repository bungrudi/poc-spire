package runguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadOnlyGuardDetectsReplacementAndDestructiveReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.json")
	cfg := Config{Path: path, Run: "run-one", Role: "caller-trusted", PodUID: "original"}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"run":"run-one","phase":"running","pods":{"caller-trusted":"original"}}`)
	if !cfg.Valid() {
		t.Fatal("valid cached guard rejected")
	}
	// No issuer/API call is part of checking a surviving valid projection.
	for _, body := range []string{
		`{"run":"run-two","phase":"running","pods":{"caller-trusted":"original"}}`,
		`{"run":"run-one","phase":"running","pods":{"caller-trusted":"replacement"}}`,
		`{"run":"run-one","phase":"invalid","pods":{"caller-trusted":"original"}}`,
		`{"run":"run-one","phase":"bootstrap","pods":{"caller-trusted":"original"}}`,
		`{broken`,
	} {
		write(body)
		if cfg.Valid() {
			t.Fatal("invalid/reset guard accepted", body)
		}
	}
	os.Remove(path)
	if cfg.Valid() {
		t.Fatal("missing guard accepted")
	}
}
