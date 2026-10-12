package main

import (
	"flag"
	"os"
	"poc-spire/internal/readiness"
)

func main() {
	path := flag.String("file", "/tmp/readiness.json", "own native readiness marker")
	flag.Parse()
	if !readiness.Check(*path) {
		os.Exit(1)
	}
}
