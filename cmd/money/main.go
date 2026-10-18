package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/kafkaio"
	"poc-spire/internal/ledger"
	"poc-spire/internal/money"
	"poc-spire/internal/readiness"
	"poc-spire/internal/runguard"
)

func main() {
	address := flag.String("listen", "127.0.0.1:8443", "listener; Compose explicitly selects its private network")
	socket := flag.String("workload", "unix:///workload/api.sock", "local Workload API")
	kafkaConfig := flag.String("kafka-config", "", "opt-in isolated Kafka configuration")
	startupTimeout := flag.Duration("startup-timeout", 15*time.Second, "bounded identity startup wait")
	readyPath := flag.String("readiness-file", "", "opt-in private readiness marker")
	guardPath := flag.String("run-guard", "", "opt-in read-only run guard")
	runID := flag.String("run", "", "expected Kubernetes run")
	flag.Parse()
	if *startupTimeout <= 0 || *startupTimeout > 180*time.Second {
		fmt.Fprintln(os.Stderr, "invalid startup bound")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, stop := context.WithTimeout(ctx, *startupTimeout)
	source, err := identity.Source(startup, *socket)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "money identity unavailable")
		os.Exit(1)
	}
	defer source.Close()
	svid, err := source.GetX509SVID()
	if err != nil || svid.ID != identity.Money {
		source.Close()
		fmt.Fprintln(os.Stderr, "unexpected money identity")
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		source.Close()
		fmt.Fprintln(os.Stderr, "money listen failed")
		os.Exit(1)
	}
	book := ledger.New(10000)
	if *kafkaConfig != "" {
		cfg, err := kafkaio.LoadConfig(*kafkaConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Kafka configuration unavailable")
			os.Exit(1)
		}
		instance, err := kafkacmd.Nonce()
		if err != nil || book.EnableKafka(instance, cfg.StreamID) != nil {
			fmt.Fprintln(os.Stderr, "Kafka instance unavailable")
			os.Exit(1)
		}
		consumer, err := kafkaio.NewConsumer(cfg, book, source, os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Kafka consumer unavailable")
			os.Exit(1)
		}
		defer consumer.Close()
		go consumer.Run(ctx)
	}
	guard := runguard.Config{Path: *guardPath, Run: *runID, Role: "money", PodUID: os.Getenv("POC_POD_UID")}
	readiness.Start(ctx, *readyPath, func(context.Context) bool {
		if !identity.Current(source) || (*guardPath != "" && !guard.Valid()) {
			return false
		}
		metadata, enabled := book.KafkaMetadata()
		return !enabled || metadata.ConsumerReady
	})
	server := money.Server(source, book, os.Stdout)
	go func() {
		<-ctx.Done()
		shutdown, end := context.WithTimeout(context.Background(), 3*time.Second)
		defer end()
		server.Shutdown(shutdown)
	}()
	server.Serve(listener)
}
