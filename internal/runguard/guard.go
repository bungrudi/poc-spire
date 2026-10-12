// Package runguard reads an operator-owned, read-only Kubernetes run guard.
package runguard

import (
	"encoding/json"
	"io"
	"os"
)

type Snapshot struct {
	Run   string            `json:"run"`
	Phase string            `json:"phase"`
	Pods  map[string]string `json:"pods"`
}

type Config struct{ Path, Run, Role, PodUID string }

func (cfg Config) Valid() bool {
	if cfg.Path == "" || cfg.Run == "" || cfg.Role == "" || cfg.PodUID == "" {
		return false
	}
	file, err := os.Open(cfg.Path)
	if err != nil {
		return false
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil || len(body) > 16384 {
		return false
	}
	var state Snapshot
	if json.Unmarshal(body, &state) != nil {
		return false
	}
	return state.Run == cfg.Run && state.Phase == "running" && state.Pods[cfg.Role] == cfg.PodUID
}
