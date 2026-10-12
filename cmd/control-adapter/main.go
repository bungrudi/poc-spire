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

	"poc-spire/internal/controladapter"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/readiness"
	"poc-spire/internal/runguard"
)

func main() {
	role := flag.String("role", "", "trusted or unidentified fixed control")
	socket := flag.String("control", "/control/api.sock", "own caller socket")
	listen := flag.String("listen", ":9001", "private adapter listener")
	guardPath := flag.String("run-guard", "", "required read-only Kubernetes run guard")
	run := flag.String("run", "", "expected disposable run")
	podUID := flag.String("pod-uid", os.Getenv("POC_POD_UID"), "expected current Pod UID")
	readyPath := flag.String("readiness-file", "", "opt-in native readiness marker")
	flag.Parse()
	expected, err := controladapter.ID(*role)
	if err != nil || *guardPath == "" || *run == "" || *podUID == "" || os.Geteuid() != 10002 || os.Getegid() != 1000 {
		fmt.Fprintln(os.Stderr, "invalid adapter deployment")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, stop := context.WithTimeout(ctx, 180*time.Second)
	source, err := identity.Source(startup, "unix:///workload/api.sock")
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "control identity unavailable")
		os.Exit(1)
	}
	defer source.Close()
	svid, err := source.GetX509SVID()
	if err != nil || svid.ID != expected {
		fmt.Fprintln(os.Stderr, "wrong control identity")
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, "control listener unavailable")
		os.Exit(1)
	}
	guard := runguard.Config{Path: *guardPath, Run: *run, Role: "caller-" + *role, PodUID: *podUID}
	readiness.Start(ctx, *readyPath, func(operation context.Context) bool {
		if !identity.Current(source) || !guard.Valid() {
			return false
		}
		m, status, err := kafkacaller.Call(operation, *socket, nil, "readiness")
		return err == nil && status == 404 && m.Initialized
	})
	server := controladapter.Server(source, controladapter.Config{Role: *role, Socket: *socket, RunValid: (runguard.Config{Path: *guardPath, Run: *run, Role: "caller-" + *role, PodUID: *podUID}).Valid}, os.Stdout)
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		server.Shutdown(shutdown)
	}()
	server.ServeTLS(listener, "", "")
}
