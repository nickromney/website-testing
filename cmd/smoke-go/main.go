package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nickromney/website-testing/internal/runner"
	"github.com/nickromney/website-testing/internal/spec"
	"github.com/nickromney/website-testing/internal/tui"
)

func main() {
	if len(os.Args) < 3 {
		usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	path := os.Args[2]

	specDoc, err := spec.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "spec load failed: %v\n", err)
		os.Exit(2)
	}

	r, err := runner.New(30 * time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "runner init failed: %v\n", err)
		os.Exit(2)
	}

	switch cmd {
	case "run":
		ok := runCLI(r, specDoc)
		if !ok {
			os.Exit(1)
		}
	case "tui":
		if err := tui.Run(specDoc, r); err != nil {
			fmt.Fprintf(os.Stderr, "tui failed: %v\n", err)
			os.Exit(2)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  smoke-go run <spec.yaml>")
	fmt.Fprintln(os.Stderr, "  smoke-go tui <spec.yaml>")
}

func runCLI(r *runner.Runner, specDoc *spec.Spec) bool {
	allOK := true
	for _, step := range specDoc.Steps {
		fmt.Printf("> %s\n", step.Name)
		res := r.RunStep(context.Background(), step)
		if res.Passed {
			fmt.Printf("  OK (%d, %s)\n", res.StatusCode, res.Duration.Round(time.Millisecond))
			continue
		}

		allOK = false
		fmt.Printf("  FAIL (%d, %s)\n", res.StatusCode, res.Duration.Round(time.Millisecond))
		for _, e := range res.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}
	return allOK
}
