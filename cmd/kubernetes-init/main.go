// kubernetes-init initializes one owned Pod's bounded disposable state only behind its run/UID barrier.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/runguard"
)

func wait(path, run, role, pod string) error {
	deadline := time.Now().Add(180 * time.Second)
	for time.Now().Before(deadline) {
		file, err := os.Open(path)
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(file, 16385))
			file.Close()
			var g runguard.Snapshot
			if readErr == nil && len(data) <= 16384 && json.Unmarshal(data, &g) == nil {
				if g.Run != run || g.Phase == "invalid" || (g.Pods[role] != "" && g.Pods[role] != pod) {
					return fmt.Errorf("run_invalid")
				}
				if g.Phase == "running" {
					return fmt.Errorf("state_initialization_refused")
				}
				if g.Pods[role] == pod && pod != "" && g.Phase == "bootstrap" {
					return nil
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("startup_guard_unavailable")
}

func main() {
	role := flag.String("role", "", "fixed component")
	run := flag.String("run", "", "expected run")
	guard := flag.String("guard", "/run/guard/run.json", "read-only guard")
	state := flag.String("state", "", "own private state directory")
	stream := flag.String("stream", "", "owned stream")
	install := flag.String("install", "", "copy bootstrap helpers")
	flag.Parse()
	if *install != "" {
		for name, binary := range map[string]string{"run-guard": "kubernetes-run-guard", "bundle-publisher": "bundle-publisher", "kubernetes-init": "kubernetes-init"} {
			data, err := os.ReadFile("/" + binary)
			if err != nil {
				os.Exit(1)
			}
			if os.WriteFile(filepath.Join(*install, name), data, 0555) != nil {
				os.Exit(1)
			}
		}
		return
	}
	if *run == "" || os.Getenv("POC_POD_UID") == "" || wait(*guard, *run, *role, os.Getenv("POC_POD_UID")) != nil {
		fmt.Fprintln(os.Stderr, "run_invalid")
		os.Exit(1)
	}
	if *role == "caller-trusted" || *role == "caller-unidentified" {
		if os.Geteuid() != 1000 || os.Getegid() != 1000 {
			os.Exit(1)
		}
		policy, err := kafkacmd.NewPolicy(*stream)
		if err != nil {
			os.Exit(1)
		}
		callerRole := "trusted"
		if *role == "caller-unidentified" {
			callerRole = "unidentified"
		}
		if err = kafkacaller.Initialize(*state, policy, callerRole); err != nil {
			fmt.Fprintln(os.Stderr, "state_initialization_refused")
			os.Exit(1)
		}
		return
	}
	if *role == "kafka" {
		marker := filepath.Join(*state, "owned-initialized")
		if _, err := os.Stat(marker); err == nil {
			fmt.Fprintln(os.Stderr, "state_initialization_refused")
			os.Exit(1)
		}
		entries, err := os.ReadDir(*state)
		if err != nil || len(entries) != 0 {
			os.Exit(1)
		}
		id, err := os.ReadFile("/config/cluster-id")
		if err != nil {
			os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, "/opt/kafka/bin/kafka-storage.sh", "format", "--config", "/config/kafka.properties", "--cluster-id", string(id))
		if command.Run() != nil {
			fmt.Fprintln(os.Stderr, "broker_initialization_failed")
			os.Exit(1)
		}
		if os.WriteFile(marker, []byte(*run+"\n"+os.Getenv("POC_POD_UID")), 0600) != nil {
			os.Exit(1)
		}
		return
	}
	os.Exit(1)
}
