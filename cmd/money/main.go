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
	"poc-spire/internal/ledger"
	"poc-spire/internal/money"
)

func main() {
	address := flag.String("listen", "127.0.0.1:8443", "listener; Compose explicitly selects its private network")
	socket := flag.String("workload", "unix:///workload/api.sock", "local Workload API")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, stop := context.WithTimeout(ctx, 15*time.Second)
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
	server := money.Server(source, ledger.New(10000), os.Stdout)
	go func() {
		<-ctx.Done()
		shutdown, end := context.WithTimeout(context.Background(), 3*time.Second)
		defer end()
		server.Shutdown(shutdown)
	}()
	server.ServeTLS(listener, "", "")
}
