package main

import (
	"fmt"
	"os"

	"github.com/nickromney/website-testing/internal/cli"
)

var (
	// Set via -ldflags at build time.
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	buildInfo := cli.BuildInfo{
		Version:   Version,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
	}

	root := cli.NewRootCmd(buildInfo)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
