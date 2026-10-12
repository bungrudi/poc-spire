// Package readiness reports native dependency status through a private bounded marker, never an anonymous Money route.
package readiness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	Ready bool  `json:"ready"`
	At    int64 `json:"at"`
}

func Start(ctx context.Context, path string, check func(context.Context) bool) {
	if path == "" {
		return
	}
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			operation, cancel := context.WithTimeout(ctx, 2*time.Second)
			state := State{Ready: check(operation), At: time.Now().Unix()}
			cancel()
			data, _ := json.Marshal(state)
			if os.WriteFile(path+".new", data, 0600) == nil {
				os.Rename(path+".new", path)
			}
			select {
			case <-ctx.Done():
				os.Remove(path)
				return
			case <-tick.C:
			}
		}
	}()
}

func Check(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 256 {
		return false
	}
	var state State
	if json.Unmarshal(data, &state) != nil {
		return false
	}
	now := time.Now().Unix()
	return state.Ready && state.At <= now && now-state.At <= 4
}
