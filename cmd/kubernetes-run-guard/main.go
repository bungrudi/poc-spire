// kubernetes-run-guard gates isolated deployment startup; it has no identity or API credentials.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"poc-spire/internal/statecheck"
	"syscall"
	"time"
)

type guard struct {
	Run   string            `json:"run"`
	Phase string            `json:"phase"`
	Pods  map[string]string `json:"pods"`
}

func readGuard(path string) (guard, error) {
	var g guard
	f, err := os.Open(path)
	if err != nil {
		return g, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil || len(b) > 16384 {
		return g, fmt.Errorf("invalid guard")
	}
	if err = json.Unmarshal(b, &g); err != nil {
		return g, fmt.Errorf("invalid guard")
	}
	return g, nil
}

const destructiveExit = 78

func terminationMessage(role, run, pod string) string {
	return "poc-spire:destructive-startup-state:" + role + ":" + run + ":" + pod
}

func destructiveState(role, run, pod string) {
	// Kubernetes mounts this dedicated file even with a read-only root. No data/key contents are exported.
	if err := os.WriteFile("/dev/termination-log", []byte(terminationMessage(role, run, pod)), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "startup_destructive_signal_unavailable")
		os.Exit(1)
	}
	os.Exit(destructiveExit)
}

func main() {
	install := flag.String("install", "", "copy this startup helper to an owned emptyDir")
	path := flag.String("guard", "/run/guard/run.json", "read-only run guard")
	role := flag.String("role", "", "fixed deployment component")
	flag.Parse()
	if *install != "" {
		self, err := os.Executable()
		if err != nil {
			panic("startup helper unavailable")
		}
		data, err := os.ReadFile(self)
		if err != nil {
			panic("startup helper unavailable")
		}
		if filepath.Base(*install) != "run-guard" {
			panic("invalid startup helper target")
		}
		if err := os.WriteFile(*install, data, 0555); err != nil {
			panic("startup helper installation failed")
		}
		return
	}
	if len(flag.Args()) == 0 || (*role == "spire-server" && flag.Args()[0] != "/opt/spire/bin/spire-server") || (*role == "kafka" && (flag.Args()[0] != "/bin/bash" || len(flag.Args()) != 2 || flag.Args()[1] != "/config/kubernetes-broker-run.sh")) || (*role != "spire-server" && *role != "kafka") {
		panic("unsupported startup role")
	}
	run, pod := os.Getenv("POC_RUN"), os.Getenv("POC_POD_UID")
	deadline := time.Now().Add(180 * time.Second)
	for {
		g, err := readGuard(*path)
		if err == nil && g.Run == run && g.Pods[*role] == pod && pod != "" {
			requiredFiles := []string{}
			if *role == "kafka" {
				requiredFiles = []string{"/var/lib/kafka/data/meta.properties", "/var/lib/kafka/data/owned-initialized"}
			}
			if g.Phase == "running" && *role == "spire-server" {
				requiredFiles = []string{"/run/spire/data/keys.json", "/run/spire/data/database.sqlite3"}
			}
			if *role == "kafka" {
				cluster, err := os.ReadFile("/config/cluster-id")
				if err != nil || len(cluster) > 128 || !statecheck.Check("kafka", "/var/lib/kafka/data", run, pod, os.Getenv("POC_STREAM"), string(cluster), g.Phase == "running").Valid {
					destructiveState(*role, run, pod)
				}
			}
			if *role == "spire-server" && g.Phase == "running" && !statecheck.Check("spire-server", "/run/spire/data", run, pod, "", "", false).Valid {
				destructiveState(*role, run, pod)
			}
			if g.Phase == "running" || g.Phase == "bootstrap" {
				for _, required := range requiredFiles {
					info, err := os.Stat(required)
					if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
						destructiveState(*role, run, pod)
					}
				}
			} else if g.Phase != "bootstrap" {
				panic("run_invalid")
			}
			if err := syscall.Exec(flag.Args()[0], flag.Args(), os.Environ()); err != nil {
				panic("component execution failed")
			}
		}
		if err == nil && (g.Run != run || g.Phase == "invalid" || (g.Pods[*role] != "" && g.Pods[*role] != pod)) {
			panic("run_invalid")
		}
		if time.Now().After(deadline) {
			panic("startup guard unavailable")
		}
		time.Sleep(200 * time.Millisecond)
	}
}
