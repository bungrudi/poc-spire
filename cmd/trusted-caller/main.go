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
	"syscall"
	"time"
)

func main() {
	path := flag.String("control", "/control/api.sock", "private trigger socket")
	target := flag.String("money", "https://money:8443", "fixed private money target")
	uid := flag.Uint("operator-uid", 10002, "private control peer UID (1000 local UI or 10002 baseline)")
	kafkaConfig := flag.String("kafka-config", "", "opt-in isolated Kafka configuration")
	flag.Parse()
	if *uid != 1000 && *uid != 10002 {
		fmt.Fprintln(os.Stderr, "unsupported control peer layout")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, stop := context.WithTimeout(ctx, 15*time.Second)
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
	var kafkaHandlers []http.Handler
	if *kafkaConfig != "" {
		sender, closeKafka, err := kafkacaller.New(*kafkaConfig, "trusted", "/workload/api.sock", source)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Kafka caller configuration unavailable")
			os.Exit(1)
		}
		defer closeKafka()
		kafkaHandlers = append(kafkaHandlers, kafkacaller.Handler(sender))
	}
	if err = control.Serve(ctx, *path, uint32(*uid), caller.Trusted(source, *target), kafkaHandlers...); err != nil {
		source.Close()
		fmt.Fprintln(os.Stderr, "trusted control unavailable")
		os.Exit(1)
	}
}
