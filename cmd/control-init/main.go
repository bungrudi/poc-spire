// control-init is trusted test/setup tooling with only fresh control volumes.
package main

import (
	"fmt"
	"os"
)

func main() {
	for _, path := range []string{"/trusted-control", "/untrusted-control"} {
		if err := os.Chown(path, 1000, 10002); err != nil {
			fmt.Fprintln(os.Stderr, "control ownership failed")
			os.Exit(1)
		}
		if err := os.Chmod(path, os.ModeSetgid|0o750); err != nil {
			fmt.Fprintln(os.Stderr, "control mode failed")
			os.Exit(1)
		}
	}
}
