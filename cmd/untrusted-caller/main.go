package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"poc-spire/internal/caller"
	"poc-spire/internal/control"
	"syscall"
)

func main() {
	path := flag.String("control", "/control/api.sock", "private trigger socket")
	target := flag.String("money", "https://money:8443", "fixed private money target")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := control.Serve(ctx, *path, 10002, caller.Unidentified("/workload/api.sock", "/workload/bundle.pem", *target)); err != nil {
		fmt.Fprintln(os.Stderr, "unidentified control unavailable")
		os.Exit(1)
	}
}
