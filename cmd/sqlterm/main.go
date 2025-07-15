package main

import (
	"fmt"
	"os"

	"sqlterm/internal/cli"
)

// Version information (set by ldflags during build)
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Set version info for CLI
	cli.SetVersionInfo(version, buildTime, gitCommit)
	
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
