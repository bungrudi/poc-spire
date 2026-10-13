// Package statecheck inspects initialized disposable stores without repairing or exporting them.
package statecheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Result struct {
	Valid       bool   `json:"valid"`
	Unavailable bool   `json:"unavailable,omitempty"`
	Incarnation string `json:"incarnation,omitempty"`
}

func read(path string, max int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > max {
		return nil, fmt.Errorf("initialized_state_unavailable")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(data)) > max {
		return nil, fmt.Errorf("initialized_state_unavailable")
	}
	return data, nil
}

func fingerprint(data []byte) Result {
	hash := sha256.Sum256(data)
	return Result{Valid: true, Incarnation: hex.EncodeToString(hash[:])}
}

func Check(role, dir, run, pod, stream, cluster string, requireTopic bool) Result {
	if run == "" || pod == "" || !filepath.IsAbs(dir) {
		return Result{}
	}
	switch role {
	case "kafka":
		marker, err := read(filepath.Join(dir, "owned-initialized"), 256)
		if err != nil || string(marker) != run+"\n"+pod {
			return Result{}
		}
		meta, err := read(filepath.Join(dir, "meta.properties"), 4096)
		if err != nil || cluster == "" {
			return Result{}
		}
		fields := map[string]string{}
		for _, line := range strings.Split(string(meta), "\n") {
			if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
				continue
			}
			pair := strings.SplitN(line, "=", 2)
			if len(pair) != 2 {
				return Result{}
			}
			name, value := strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])
			if _, exists := fields[name]; exists {
				return Result{}
			}
			fields[name] = value
		}
		if fields["cluster.id"] != cluster || fields["node.id"] != "1" || fields["version"] != "1" {
			return Result{}
		}
		if requireTopic {
			if !validStream(stream) {
				return Result{}
			}
			for _, name := range []string{"__cluster_metadata-0", "poc.spire.commands." + stream + "-0"} {
				info, err := os.Lstat(filepath.Join(dir, name))
				if err != nil || !info.IsDir() {
					return Result{}
				}
			}
		}
		return fingerprint(append(meta, marker...))
	case "spire-server":
		keys, err := read(filepath.Join(dir, "keys.json"), 1<<20)
		var decoded map[string]json.RawMessage
		if err != nil || json.Unmarshal(keys, &decoded) != nil || len(decoded) == 0 {
			return Result{}
		}
		// SQLite files may be large. Read only the format header; never allocate the database.
		f, err := os.Open(filepath.Join(dir, "database.sqlite3"))
		if err != nil {
			return Result{}
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() < 100 {
			return Result{}
		}
		header := make([]byte, 16)
		if _, err = io.ReadFull(f, header); err != nil || string(header) != "SQLite format 3\x00" {
			return Result{}
		}
		return fingerprint([]byte(run + "\n" + pod))
	}
	return Result{}
}

func validStream(stream string) bool {
	if len(stream) != 32 {
		return false
	}
	for _, c := range stream {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
