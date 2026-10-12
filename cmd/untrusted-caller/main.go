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
	"poc-spire/internal/kafkacaller"
	"syscall"
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
	var kafkaHandlers []http.Handler
	if *kafkaConfig != "" {
		sender, closeKafka, err := kafkacaller.New(*kafkaConfig, "unidentified", "/workload/api.sock", nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Kafka caller configuration unavailable")
			os.Exit(1)
		}
		defer closeKafka()
		kafkaHandlers = append(kafkaHandlers, kafkacaller.Handler(sender))
	}
	if err := control.Serve(ctx, *path, uint32(*uid), caller.Unidentified("/workload/api.sock", "/workload/bundle.pem", *target), kafkaHandlers...); err != nil {
		fmt.Fprintln(os.Stderr, "unidentified control unavailable")
		os.Exit(1)
	}
}
