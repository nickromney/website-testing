package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nickromney/website-testing/internal/runner"
	"github.com/nickromney/website-testing/internal/spec"
	"github.com/nickromney/website-testing/internal/tui"
	"github.com/spf13/cobra"
)

const (
	rootLong = `smoke-go runs declarative YAML smoke specs for website and network checks.

It supports four step kinds:
  - http: request/response assertions
  - dns: A, AAAA, CNAME, MX, NS, and TXT lookups
  - tls: certificate expiry and peer certificate inspection
  - tcp: raw TCP connectivity checks

Specs are YAML files with a list of steps. Each step declares a kind-specific
target plus expectations. Use 'run' for plain terminal or JSON output, and use
'tui' for an interactive Bubble Tea view with reruns and per-step details.`

	rootExample = `  smoke-go run examples/http.yaml
  smoke-go run examples/network.yaml --json
  smoke-go tui examples/network.yaml
  smoke-go run my-checks.yaml --timeout 10s`

	runLong = `Run executes every step in a YAML smoke spec and prints a human summary.

Use --json when you want a machine-readable result object for automation. The
command exits non-zero if any step fails or if the spec cannot be loaded.`

	runExample = `  smoke-go run examples/http.yaml
  smoke-go run examples/network.yaml
  smoke-go run examples/network.yaml --json
  smoke-go run prod.yaml --timeout 15s`

	tuiLong = `TUI runs a YAML smoke spec in an interactive terminal interface.

The TUI shows a step list, detailed request/response or network probe data, and
lets you rerun either the selected step or the full spec. It requires an
interactive TTY.`

	tuiExample = `  smoke-go tui examples/network.yaml
  smoke-go tui prod.yaml --timeout 15s`
)

type BuildInfo struct {
	Version   string
	BuildTime string
	GitCommit string
}

type runOutput struct {
	Spec    string         `json:"spec"`
	Passed  bool           `json:"passed"`
	Results []resultOutput `json:"results"`
}

type resultOutput struct {
	Kind        string              `json:"kind"`
	Name        string              `json:"name"`
	Method      string              `json:"method"`
	URL         string              `json:"url"`
	StatusCode  int                 `json:"status_code"`
	DurationMS  int64               `json:"duration_ms"`
	Passed      bool                `json:"passed"`
	Errors      []string            `json:"errors,omitempty"`
	Headers     map[string][]string `json:"headers,omitempty"`
	BodyPreview string              `json:"body_preview,omitempty"`
	DNSName     string              `json:"dns_name,omitempty"`
	DNSType     string              `json:"dns_type,omitempty"`
	DNSServer   string              `json:"dns_server,omitempty"`
	DNSAnswers  []string            `json:"dns_answers,omitempty"`
	TLSAddress  string              `json:"tls_address,omitempty"`
	TLSServer   string              `json:"tls_server_name,omitempty"`
	TLSSubject  string              `json:"tls_subject,omitempty"`
	TLSIssuer   string              `json:"tls_issuer,omitempty"`
	TLSNotAfter string              `json:"tls_not_after,omitempty"`
	TLSDays     *int                `json:"tls_days_remaining,omitempty"`
	TCPAddress  string              `json:"tcp_address,omitempty"`
}

func NewRootCmd(buildInfo BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:           "smoke-go",
		Short:         "HTTP, DNS, TLS, and TCP smoke testing runner",
		Long:          rootLong,
		Example:       rootExample,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
	}

	root.Version = buildInfo.Version + "\nbuild_time: " + buildInfo.BuildTime + "\ngit_commit: " + buildInfo.GitCommit
	root.SetVersionTemplate("smoke-go {{.Version}}\n")

	root.AddCommand(
		buildRunCmd(),
		buildTUICmd(),
		buildVersionCmd(buildInfo),
	)

	root.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}

	return root
}

func buildRunCmd() *cobra.Command {
	var (
		timeout time.Duration
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "run SPEC",
		Short:   "Run a smoke spec in the terminal",
		Long:    runLong,
		Example: runExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath := args[0]
			specDoc, err := spec.Load(specPath)
			if err != nil {
				return fmt.Errorf("spec load failed: %w", err)
			}

			r, err := runner.New(timeout)
			if err != nil {
				return fmt.Errorf("runner init failed: %w", err)
			}

			output := runOutput{
				Spec:    specPath,
				Passed:  true,
				Results: make([]resultOutput, 0, len(specDoc.Steps)),
			}

			for _, step := range specDoc.Steps {
				res := r.RunStep(context.Background(), step)
				output.Results = append(output.Results, toResultOutput(res))

				if !res.Passed {
					output.Passed = false
				}

				if jsonOut {
					continue
				}

				fmt.Fprintf(cmd.OutOrStdout(), "> %s\n", step.Name)
				if res.Passed {
					fmt.Fprintf(cmd.OutOrStdout(), "  OK (%s)\n", humanSummary(res))
					continue
				}

				fmt.Fprintf(cmd.OutOrStdout(), "  FAIL (%s)\n", humanSummary(res))
				for _, e := range res.Errors {
					fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", e)
				}
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				if err := enc.Encode(output); err != nil {
					return err
				}
			}

			if !output.Passed {
				return fmt.Errorf("one or more steps failed")
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Per-step network timeout")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit machine-readable JSON to stdout")

	return cmd
}

func buildTUICmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:     "tui SPEC",
		Short:   "Run a smoke spec in the interactive TUI",
		Long:    tuiLong,
		Example: tuiExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath := args[0]
			specDoc, err := spec.Load(specPath)
			if err != nil {
				return fmt.Errorf("spec load failed: %w", err)
			}

			if !isInteractiveTTY() {
				return fmt.Errorf("TUI requires an interactive TTY")
			}

			r, err := runner.New(timeout)
			if err != nil {
				return fmt.Errorf("runner init failed: %w", err)
			}

			if err := tui.Run(specDoc, r, specPath); err != nil {
				return fmt.Errorf("tui failed: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Per-step network timeout")

	return cmd
}

func buildVersionCmd(buildInfo BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show build information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "smoke-go %s\n", buildInfo.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "build_time: %s\n", buildInfo.BuildTime)
			fmt.Fprintf(cmd.OutOrStdout(), "git_commit: %s\n", buildInfo.GitCommit)
		},
	}
}

func toResultOutput(res runner.Result) resultOutput {
	out := resultOutput{
		Kind:       res.Kind,
		Name:       res.Step.Name,
		Method:     res.Step.Request.Method,
		URL:        res.Step.Request.URL,
		StatusCode: res.StatusCode,
		DurationMS: res.Duration.Milliseconds(),
		Passed:     res.Passed,
		Errors:     append([]string(nil), res.Errors...),
		Headers:    res.Headers.Clone(),
		DNSName:    res.DNSName,
		DNSType:    res.DNSRecordType,
		DNSServer:  res.DNSResolver,
		DNSAnswers: append([]string(nil), res.DNSAnswers...),
		TLSAddress: res.TLSAddress,
		TLSServer:  res.TLSServerName,
		TLSSubject: res.TLSSubject,
		TLSIssuer:  res.TLSIssuer,
		TCPAddress: res.TCPAddress,
	}
	if !res.TLSNotAfter.IsZero() {
		out.TLSNotAfter = res.TLSNotAfter.Format(time.RFC3339)
		days := res.TLSDaysRemaining
		out.TLSDays = &days
	}

	text := strings.ReplaceAll(string(res.Body), "\x00", "\uFFFD")
	if len(text) > 1200 {
		text = text[:1200] + "\n...[truncated]"
	}
	if strings.TrimSpace(text) != "" {
		out.BodyPreview = text
	}

	return out
}

func humanSummary(res runner.Result) string {
	duration := res.Duration.Round(time.Millisecond)
	switch res.Kind {
	case spec.KindDNS:
		return fmt.Sprintf("%d answer(s), %s", len(res.DNSAnswers), duration)
	case spec.KindTLS:
		return fmt.Sprintf("%d day(s) remaining, %s", res.TLSDaysRemaining, duration)
	case spec.KindTCP:
		return fmt.Sprintf("%s, %s", res.TCPAddress, duration)
	default:
		return fmt.Sprintf("%d, %s", res.StatusCode, duration)
	}
}

func isInteractiveTTY() bool {
	stdin, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	stdout, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (stdin.Mode()&os.ModeCharDevice) != 0 && (stdout.Mode()&os.ModeCharDevice) != 0
}
