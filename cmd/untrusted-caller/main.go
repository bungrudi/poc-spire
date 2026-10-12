package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"net/http"
	"os"
	"os/signal"
	"poc-spire/internal/caller"
	"poc-spire/internal/control"
	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/readiness"
	"poc-spire/internal/runguard"
	"syscall"
	"time"
)

func main() {
	path := flag.String("control", "/control/api.sock", "private trigger socket")
	target := flag.String("money", "https://money:8443", "fixed private money target")
	uid := flag.Uint("operator-uid", 10002, "private control peer UID (1000 local UI or 10002 baseline)")
	kafkaConfig := flag.String("kafka-config", "", "opt-in isolated Kafka configuration")
	bundlePath := flag.String("public-bundle", "/workload/bundle.pem", "authentic public SPIRE trust file")
	readyPath := flag.String("readiness-file", "", "opt-in private native readiness marker")
	guardPath := flag.String("run-guard", "", "opt-in read-only run guard")
	runID := flag.String("run", "", "expected run")
	reopen := flag.Bool("reopen-control", false, "opt-in owned Kubernetes socket reopening")
	flag.Parse()
	if *uid != 1000 && *uid != 10002 {
		fmt.Fprintln(os.Stderr, "unsupported control peer layout")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var senderForReady *kafkacaller.Sender
	var kafkaHandlers []http.Handler
	if *kafkaConfig != "" {
		sender, closeKafka, err := kafkacaller.New(*kafkaConfig, "unidentified", "/workload/api.sock", nil)
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
	guard := runguard.Config{Path: *guardPath, Run: *runID, Role: "caller-unidentified", PodUID: os.Getenv("POC_POD_UID")}
	readiness.Start(ctx, *readyPath, func(operation context.Context) bool {
		if !(true) || (*guardPath != "" && !guard.Valid()) {
			return false
		}

		bundle, err := x509bundle.Load(spiffeid.RequireTrustDomainFromString("poc.internal"), *bundlePath)
		if err != nil {
			return false
		}
		current := false
		now := time.Now()
		for _, ca := range bundle.X509Authorities() {
			if !now.Before(ca.NotBefore) && now.Before(ca.NotAfter) {
				current = true
			}
		}
		if !current {
			return false
		}
		if kafkacaller.NoIdentity("/workload/api.sock")(operation) != nil {
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
	if err := control.Serve(ctx, *path, uint32(*uid), caller.Unidentified("/workload/api.sock", *bundlePath, *target), kafkaHandlers...); err != nil {
		fmt.Fprintln(os.Stderr, "unidentified control unavailable")
		os.Exit(1)
	}
}
