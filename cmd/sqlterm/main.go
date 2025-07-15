package main

import (
	"fmt"
	"os"

	"sqlterm/internal/cli"
	"sqlterm/internal/i18n"
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
		// Try to initialize i18n for error message
		i18nMgr, i18nErr := i18n.NewManager("en_au")
		if i18nErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, i18nMgr.Get("main_error"), err)
		}
		os.Exit(1)
	}
}
