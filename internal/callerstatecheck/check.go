// Package callerstatecheck validates original bounded caller data without repairing it.
package callerstatecheck

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/statecheck"
)

func Check(role, dir, run, pod, stream string) statecheck.Result {
	if run == "" || pod == "" || !filepath.IsAbs(dir) || (role != "caller-trusted" && role != "caller-unidentified") {
		return statecheck.Result{}
	}
	policy, err := kafkacmd.NewPolicy(stream)
	if err != nil {
		return statecheck.Result{}
	}
	callerRole := "trusted"
	if role == "caller-unidentified" {
		callerRole = "unidentified"
	}
	if _, err = kafkacaller.Open(dir, policy, callerRole); err != nil {
		if err.Error() == "caller_state_busy" {
			return statecheck.Result{Unavailable: true}
		}
		return statecheck.Result{}
	}
	f, err := os.Open(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return statecheck.Result{}
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1025))
	if err != nil || len(data) > 1024 {
		return statecheck.Result{}
	}
	hash := sha256.Sum256(data)
	return statecheck.Result{Valid: true, Incarnation: hex.EncodeToString(hash[:])}
}
