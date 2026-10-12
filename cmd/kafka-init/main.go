// kafka-init is runner tooling; caller startup never invokes initialization.
package main

import (
	"flag"
	"fmt"
	"os"
	"poc-spire/internal/kafkacaller"
	"poc-spire/internal/kafkacmd"
)

func main() {
	dir := flag.String("state", "", "new private caller directory")
	stream := flag.String("stream", "", "32-hex run stream")
	role := flag.String("role", "", "trusted or unidentified")
	flag.Parse()
	policy, err := kafkacmd.NewPolicy(*stream)
	if err == nil {
		err = kafkacaller.Initialize(*dir, policy, *role)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "caller initialization failed")
		os.Exit(1)
	}
}
