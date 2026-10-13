// Package readiness reports native dependency status through a private bounded marker, never an anonymous Money route.
package readiness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const markerLimit = 512

// Incarnation distinguishes the process from a previous container that shared its emptyDir.
// Exec probes use the container's own PID namespace, including during early startup.
type Incarnation struct {
	PID       int    `json:"pid"`
	Start     uint64 `json:"start"`
	Namespace string `json:"namespace"`
}

type State struct {
	Ready   bool        `json:"ready"`
	At      int64       `json:"at"`
	Process Incarnation `json:"process"`
}

func boundedFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("invalid readiness file")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, fmt.Errorf("invalid readiness file")
	}
	return data, nil
}

func process(pid int) (Incarnation, error) {
	if pid <= 0 {
		return Incarnation{}, fmt.Errorf("invalid readiness process")
	}
	base := "/proc/" + strconv.Itoa(pid)
	data, err := boundedFile(base+"/stat", 4096)
	if err != nil {
		return Incarnation{}, err
	}
	// comm can contain spaces and parentheses; subsequent fields begin after its final ')'.
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return Incarnation{}, fmt.Errorf("invalid process stat")
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) <= 19 || fields[0] == "Z" || fields[0] == "X" {
		return Incarnation{}, fmt.Errorf("process unavailable")
	}
	start, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || start == 0 {
		return Incarnation{}, fmt.Errorf("invalid process start")
	}
	namespace, err := os.Readlink(base + "/ns/pid")
	if err != nil || !strings.HasPrefix(namespace, "pid:[") || len(namespace) > 64 {
		return Incarnation{}, fmt.Errorf("invalid process namespace")
	}
	return Incarnation{PID: pid, Start: start, Namespace: namespace}, nil
}

func Start(ctx context.Context, path string, check func(context.Context) bool) {
	if path == "" {
		return
	}
	owner, err := process(os.Getpid())
	if err != nil {
		os.Remove(path)
		return
	}
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			operation, cancel := context.WithTimeout(ctx, 2*time.Second)
			state := State{Ready: check(operation), At: time.Now().Unix(), Process: owner}
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
	data, err := boundedFile(path, markerLimit)
	if err != nil {
		return false
	}
	var state State
	if json.Unmarshal(data, &state) != nil {
		return false
	}
	now := time.Now().Unix()
	if !state.Ready || state.At > now || now-state.At > 4 {
		return false
	}
	owner, err := process(state.Process.PID)
	return err == nil && owner == state.Process
}
