package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"poc-spire/internal/caller"
	"poc-spire/internal/control"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/readiness"
	"poc-spire/internal/runguard"
	"syscall"
	"time"
)

func main() {
	path := flag.String("control", "/control/api.sock", "private trigger socket")
	target := flag.String("money", "http://money:8443", "fixed private money target")
	uid := flag.Uint("operator-uid", 10002, "private control peer UID (1000 local UI or 10002 baseline)")
	kafkaConfig := flag.String("kafka-config", "", "opt-in isolated Kafka configuration")
	startupTimeout := flag.Duration("startup-timeout", 15*time.Second, "bounded identity startup wait")
	readyPath := flag.String("readiness-file", "", "opt-in private native readiness marker")
	guardPath := flag.String("run-guard", "", "opt-in read-only run guard")
	runID := flag.String("run", "", "expected run")
	reopen := flag.Bool("reopen-control", false, "opt-in owned Kubernetes socket reopening")
	flag.Parse()
	if *startupTimeout <= 0 || *startupTimeout > 180*time.Second {
		fmt.Fprintln(os.Stderr, "invalid startup bound")
		os.Exit(1)
	}
	if *uid != 1000 && *uid != 10002 {
		fmt.Fprintln(os.Stderr, "unsupported control peer layout")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if fixtureBeforeSource(ctx) {
		return
	}
	startup, stop := context.WithTimeout(ctx, *startupTimeout)
	source, err := identity.Source(startup, "unix:///workload/api.sock")
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "trusted identity unavailable")
		os.Exit(1)
	}
	defer source.Close()
	svid, err := source.GetX509SVID()
	if err != nil || svid.ID != identity.Trusted {
		source.Close()
		fmt.Fprintln(os.Stderr, "wrong trusted identity")
		os.Exit(1)
	}
	if fixtureRun(ctx, source) {
		return
	}
	var senderForReady *kafkacaller.Sender
	var kafkaHandlers []http.Handler
	if *kafkaConfig != "" {
		sender, closeKafka, err := kafkacaller.New(*kafkaConfig, "trusted", "/workload/api.sock", source)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Kafka caller configuration unavailable")
			os.Exit(1)
		}
		senderForReady = sender
		defer closeKafka()
		kafkaHandlers = append(kafkaHandlers, kafkacaller.Handler(sender))
	}
	if *reopen {
		if control.ReopenOwnSocket(*path) != nil {
			fmt.Fprintln(os.Stderr, "control reopening refused")
			os.Exit(1)
		}
	}
	guard := runguard.Config{Path: *guardPath, Run: *runID, Role: "caller-trusted", PodUID: os.Getenv("POC_POD_UID")}
	readiness.Start(ctx, *readyPath, func(operation context.Context) bool {
		if !(identity.Current(source)) || (*guardPath != "" && !guard.Valid()) {
			return false
		}

		if senderForReady != nil {
			m, _, err := senderForReady.Store.Lookup("readiness")
			if err != nil || !m.Initialized {
				return false
			}
		}
		info, err := os.Lstat(*path)
		return err == nil && info.Mode()&os.ModeSocket != 0
	})
	execute := caller.Trusted(source, *target)
	if err = control.Serve(ctx, *path, uint32(*uid), execute, kafkaHandlers...); err != nil {
		source.Close()
		fmt.Fprintln(os.Stderr, "trusted control unavailable")
		os.Exit(1)
	}
}
