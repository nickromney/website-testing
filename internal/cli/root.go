package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nickromney/website-testing/internal/adhoc"
	"github.com/nickromney/website-testing/internal/runner"
	"github.com/nickromney/website-testing/internal/spec"
	"github.com/nickromney/website-testing/internal/tui"
	"github.com/spf13/cobra"
)

const (
	rootLong = `smoke-go has two modes:

  1. Shell-style ad hoc checks:
     pass a target plus flags and it behaves like a simple smoke script.
     Example: smoke-go google.com

  2. Config-driven checks:
     run a YAML spec for HTTP, DNS, TLS, and TCP steps, or launch the TUI.

Bare targets default to HTTPS, perform a GET request, and expect a 200
response unless you override that with flags.`

	rootExample = `  smoke-go google.com
  smoke-go https://example.com/health --status 200 --body-contains Example
  smoke-go http://localhost:3000 --header "Host: example.test"
  smoke-go run examples/network.yaml --json
  smoke-go tui
  smoke-go tui examples/network.yaml`

	runLong = `Run executes every step in a YAML smoke spec and prints a human summary.

Use --json when you want a machine-readable result object for automation. The
command exits non-zero if any step fails or if the spec cannot be loaded.`

	runExample = `  smoke-go run examples/http.yaml
  smoke-go run examples/network.yaml
  smoke-go run examples/network.yaml --json
  smoke-go run prod.yaml --timeout 15s`

	tuiLong = `TUI has two modes:

  - with SPEC: inspect and rerun an existing YAML smoke spec
  - without SPEC: interactively build a one-step HTTP smoke check, then run it

The builder mode is intended to replace the "edit a shell script by hand"
workflow for simple checks.`

	tuiExample = `  smoke-go tui
  smoke-go tui examples/network.yaml
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

type humanCheck struct {
	Passed bool
	Text   string
}

type quickOptions struct {
	timeout        time.Duration
	jsonOut        bool
	method         string
	status         int
	body           string
	headers        []string
	bodyContains   []string
	bodyAbsent     []string
	headerContains []string
}

func NewRootCmd(buildInfo BuildInfo) *cobra.Command {
	opts := quickOptions{}

	root := &cobra.Command{
		Use:           "smoke-go [TARGET]",
		Short:         "Shell-style smoke checks plus YAML/TUI workflows",
		Long:          rootLong,
		Example:       rootExample,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			arg := args[0]
			if isSpecPath(arg) {
				specDoc, err := spec.Load(arg)
				if err != nil {
					return fmt.Errorf("spec load failed: %w", err)
				}
				return runSpecDocument(cmd, arg, specDoc, opts.timeout, opts.jsonOut)
			}

			headers, err := parseHeaders(opts.headers)
			if err != nil {
				return err
			}
			step, err := adhoc.HTTPCheck(arg, adhoc.HTTPOptions{
				Method:         opts.method,
				Status:         opts.status,
				Body:           opts.body,
				Headers:        headers,
				BodyContains:   opts.bodyContains,
				BodyAbsent:     opts.bodyAbsent,
				HeaderContains: opts.headerContains,
			})
			if err != nil {
				return err
			}

			specDoc := &spec.Spec{
				Name:  "quick smoke",
				Steps: []spec.Step{step},
			}
			return runSpecDocument(cmd, arg, specDoc, opts.timeout, opts.jsonOut)
		},
	}

	root.Version = buildInfo.Version + "\nbuild_time: " + buildInfo.BuildTime + "\ngit_commit: " + buildInfo.GitCommit
	root.SetVersionTemplate("smoke-go {{.Version}}\n")
	root.Flags().DurationVar(&opts.timeout, "timeout", 30*time.Second, "Per-step network timeout")
	root.Flags().BoolVar(&opts.jsonOut, "json", false, "Emit machine-readable JSON to stdout")
	root.Flags().StringVar(&opts.method, "method", "GET", "HTTP method for ad hoc target mode")
	root.Flags().IntVar(&opts.status, "status", 200, "Expected HTTP status for ad hoc target mode")
	root.Flags().StringVar(&opts.body, "body", "", "HTTP request body for ad hoc target mode")
	root.Flags().StringArrayVarP(&opts.headers, "header", "H", nil, "HTTP request header in 'Name: value' form (repeatable)")
	root.Flags().StringArrayVar(&opts.bodyContains, "body-contains", nil, "Assert that the response body contains this string (repeatable)")
	root.Flags().StringArrayVar(&opts.bodyAbsent, "body-absent", nil, "Assert that the response body does not contain this string (repeatable)")
	root.Flags().StringArrayVar(&opts.headerContains, "header-contains", nil, "Assert that the response headers contain this string (repeatable)")

	root.AddCommand(
		buildRunCmd(),
		buildTUICmd(),
		buildVersionCmd(buildInfo),
	)

	return root
}

func buildRunCmd() *cobra.Command {
	var (
		timeout time.Duration
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "run SPEC",
		Short:   "Run a YAML smoke spec in the terminal",
		Long:    runLong,
		Example: runExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath := args[0]
			specDoc, err := spec.Load(specPath)
			if err != nil {
				return fmt.Errorf("spec load failed: %w", err)
			}
			return runSpecDocument(cmd, specPath, specDoc, timeout, jsonOut)
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Per-step network timeout")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit machine-readable JSON to stdout")

	return cmd
}

func buildTUICmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:     "tui [SPEC]",
		Short:   "Open the interactive builder or run an existing spec in the TUI",
		Long:    tuiLong,
		Example: tuiExample,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isInteractiveTTY() {
				return fmt.Errorf("TUI requires an interactive TTY")
			}

			r, err := runner.New(timeout)
			if err != nil {
				return fmt.Errorf("runner init failed: %w", err)
			}

			if len(args) == 0 {
				if err := tui.RunBuilder(r); err != nil {
					return fmt.Errorf("tui failed: %w", err)
				}
				return nil
			}

			specPath := args[0]
			specDoc, err := spec.Load(specPath)
			if err != nil {
				return fmt.Errorf("spec load failed: %w", err)
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

func runSpecDocument(cmd *cobra.Command, source string, specDoc *spec.Spec, timeout time.Duration, jsonOut bool) error {
	r, err := runner.New(timeout)
	if err != nil {
		return fmt.Errorf("runner init failed: %w", err)
	}

	output := runOutput{
		Spec:    source,
		Passed:  true,
		Results: make([]resultOutput, 0, len(specDoc.Steps)),
	}
	totalChecks := 0
	failedChecks := 0

	for _, step := range specDoc.Steps {
		res := r.RunStep(context.Background(), step)
		output.Results = append(output.Results, toResultOutput(res))

		if !res.Passed {
			output.Passed = false
		}

		if jsonOut {
			continue
		}

		checks := humanChecks(res)
		totalChecks += len(checks)
		for _, check := range checks {
			if !check.Passed {
				failedChecks++
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "> %s\n", humanStepHeading(res))
		for _, check := range checks {
			fmt.Fprintf(cmd.OutOrStdout(), "    %s %s\n", humanCheckStatus(check.Passed), check.Text)
		}
	}

	if jsonOut {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(output); err != nil {
			return err
		}
	} else {
		if failedChecks > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "FAIL (%d/%d)\n", failedChecks, totalChecks)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "OK (%d/%d)\n", totalChecks, totalChecks)
		}
	}

	if !output.Passed {
		return fmt.Errorf("one or more steps failed")
	}
	return nil
}

func isSpecPath(arg string) bool {
	clean := strings.TrimSpace(arg)
	if clean == "" {
		return false
	}

	ext := strings.ToLower(filepath.Ext(clean))
	if ext != ".yaml" && ext != ".yml" {
		return false
	}

	info, err := os.Stat(clean)
	return err == nil && !info.IsDir()
}

func parseHeaders(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	headers := make(map[string]string, len(values))
	for _, value := range values {
		name, headerValue, ok := strings.Cut(value, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q: expected 'Name: value'", value)
		}
		name = strings.TrimSpace(name)
		headerValue = strings.TrimSpace(headerValue)
		if name == "" {
			return nil, fmt.Errorf("invalid header %q: header name is empty", value)
		}
		headers[name] = headerValue
	}
	return headers, nil
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

func humanChecks(res runner.Result) []humanCheck {
	switch res.Kind {
	case spec.KindDNS:
		return humanDNSChecks(res)
	case spec.KindTLS:
		return humanTLSChecks(res)
	case spec.KindTCP:
		return humanTCPChecks(res)
	default:
		return humanHTTPChecks(res)
	}
}

func humanHTTPChecks(res runner.Result) []humanCheck {
	runtimeErrs := runtimeErrors(res.Errors)
	if res.StatusCode == 0 && len(runtimeErrs) > 0 {
		return failureChecks("HTTP request failed: ", runtimeErrs)
	}

	var checks []humanCheck
	if res.Step.Expect.Status != 0 {
		text := fmt.Sprintf("%d Response code", res.Step.Expect.Status)
		if res.StatusCode != res.Step.Expect.Status {
			text = fmt.Sprintf("%d Response code (got %d)", res.Step.Expect.Status, res.StatusCode)
		}
		checks = append(checks, humanCheck{
			Passed: res.StatusCode == res.Step.Expect.Status,
			Text:   text,
		})
	}

	checks = append(checks, humanBodyChecks(res.Step.Expect, string(res.Body))...)
	checks = append(checks, humanHeaderChecks(res.Step.Expect, res.Headers)...)
	checks = append(checks, failureChecks("HTTP request error: ", runtimeErrs)...)

	if len(checks) == 0 {
		checks = append(checks, humanCheck{
			Passed: res.StatusCode > 0,
			Text:   fmt.Sprintf("%d Response code", res.StatusCode),
		})
	}
	return checks
}

func humanDNSChecks(res runner.Result) []humanCheck {
	runtimeErrs := runtimeErrors(res.Errors)
	if len(runtimeErrs) > 0 && len(res.DNSAnswers) == 0 {
		return failureChecks("DNS lookup failed: ", runtimeErrs)
	}

	var checks []humanCheck
	checks = append(checks, humanBodyChecks(res.Step.Expect, string(res.Body))...)
	answerText := strings.Join(res.DNSAnswers, "\n")
	for _, s := range res.Step.Expect.AnswerContains {
		if s == "" {
			continue
		}
		checks = append(checks, humanCheck{
			Passed: strings.Contains(answerText, s),
			Text:   humanContainsText(strings.Contains(answerText, s), "Dig results", s),
		})
	}
	for _, s := range res.Step.Expect.AnswerAbsent {
		if s == "" {
			continue
		}
		contains := strings.Contains(answerText, s)
		checks = append(checks, humanCheck{
			Passed: !contains,
			Text:   humanAbsentText(!contains, "Dig results", s),
		})
	}
	checks = append(checks, failureChecks("DNS lookup error: ", runtimeErrs)...)
	if len(checks) == 0 {
		checks = append(checks, humanCheck{
			Passed: true,
			Text:   fmt.Sprintf("%s lookup returned %d answer(s)", strings.ToUpper(res.DNSRecordType), len(res.DNSAnswers)),
		})
	}
	return checks
}

func humanTLSChecks(res runner.Result) []humanCheck {
	runtimeErrs := runtimeErrors(res.Errors)
	if len(runtimeErrs) > 0 && res.TLSNotAfter.IsZero() {
		return failureChecks("TLS handshake failed: ", runtimeErrs)
	}

	var checks []humanCheck
	checks = append(checks, humanBodyChecks(res.Step.Expect, string(res.Body))...)
	if res.Step.Expect.DaysRemainingAtLeast != nil {
		minimum := *res.Step.Expect.DaysRemainingAtLeast
		passed := res.TLSDaysRemaining >= minimum
		text := fmt.Sprintf("Certificate valid for %d days", res.TLSDaysRemaining)
		if !passed {
			text = fmt.Sprintf("Certificate expires in %d days (need at least %d)", res.TLSDaysRemaining, minimum)
		}
		checks = append(checks, humanCheck{Passed: passed, Text: text})
	}
	checks = append(checks, failureChecks("TLS error: ", runtimeErrs)...)
	if len(checks) == 0 {
		checks = append(checks, humanCheck{Passed: true, Text: "TLS handshake succeeded"})
	}
	return checks
}

func humanTCPChecks(res runner.Result) []humanCheck {
	runtimeErrs := runtimeErrors(res.Errors)
	if len(runtimeErrs) > 0 && len(res.Body) == 0 {
		return failureChecks("TCP connection failed: ", runtimeErrs)
	}

	checks := humanBodyChecks(res.Step.Expect, string(res.Body))
	checks = append(checks, failureChecks("TCP error: ", runtimeErrs)...)
	if len(checks) == 0 {
		checks = append(checks, humanCheck{Passed: true, Text: "TCP connection succeeded"})
	}
	return checks
}

func humanStepHeading(res runner.Result) string {
	switch res.Kind {
	case spec.KindDNS:
		return res.DNSName
	case spec.KindTLS:
		return res.TLSAddress
	case spec.KindTCP:
		return res.TCPAddress
	default:
		return res.Step.Request.URL
	}
}

func humanCheckStatus(passed bool) string {
	if passed {
		return "[ OK ]"
	}
	return "[FAIL]"
}

func humanBodyChecks(expect spec.Expect, body string) []humanCheck {
	var checks []humanCheck
	for _, s := range expect.BodyContains {
		if s == "" {
			continue
		}
		contains := strings.Contains(body, s)
		checks = append(checks, humanCheck{
			Passed: contains,
			Text:   humanContainsText(contains, "Body", s),
		})
	}
	for _, s := range expect.BodyAbsent {
		if s == "" {
			continue
		}
		contains := strings.Contains(body, s)
		checks = append(checks, humanCheck{
			Passed: !contains,
			Text:   humanAbsentText(!contains, "Body", s),
		})
	}
	return checks
}

func humanHeaderChecks(expect spec.Expect, headers map[string][]string) []humanCheck {
	if len(expect.HeaderHas) == 0 {
		return nil
	}

	var b strings.Builder
	for k, values := range headers {
		for _, value := range values {
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(value)
			b.WriteByte('\n')
		}
	}
	headerText := b.String()

	checks := make([]humanCheck, 0, len(expect.HeaderHas))
	for _, s := range expect.HeaderHas {
		if s == "" {
			continue
		}
		contains := strings.Contains(headerText, s)
		checks = append(checks, humanCheck{
			Passed: contains,
			Text:   humanContainsText(contains, "Headers", s),
		})
	}
	return checks
}

func humanContainsText(passed bool, noun string, value string) string {
	positive, negative := containsVerbs(noun)
	if passed {
		return fmt.Sprintf("%s %s %q", noun, positive, value)
	}
	return fmt.Sprintf("%s %s %q", noun, negative, value)
}

func humanAbsentText(passed bool, noun string, value string) string {
	positive, negative := containsVerbs(noun)
	if passed {
		return fmt.Sprintf("(Assert absence of string): %s %s %q", noun, negative, value)
	}
	return fmt.Sprintf("(Assert absence of string): %s %s %q", noun, positive, value)
}

func containsVerbs(noun string) (positive string, negative string) {
	switch noun {
	case "Body":
		return "contains", "does not contain"
	default:
		return "contain", "do not contain"
	}
}

func runtimeErrors(errors []string) []string {
	runtime := make([]string, 0, len(errors))
	for _, err := range errors {
		if isExpectationError(err) {
			continue
		}
		runtime = append(runtime, err)
	}
	return runtime
}

func isExpectationError(err string) bool {
	switch {
	case strings.HasPrefix(err, "expected status "):
		return true
	case strings.HasPrefix(err, "expected body to contain "):
		return true
	case strings.HasPrefix(err, "expected body to NOT contain "):
		return true
	case strings.HasPrefix(err, "expected headers to contain "):
		return true
	case strings.HasPrefix(err, "expected answers to contain "):
		return true
	case strings.HasPrefix(err, "expected answers to NOT contain "):
		return true
	case strings.HasPrefix(err, "expected days remaining >= "):
		return true
	default:
		return false
	}
}

func failureChecks(prefix string, errs []string) []humanCheck {
	checks := make([]humanCheck, 0, len(errs))
	for _, err := range errs {
		checks = append(checks, humanCheck{
			Passed: false,
			Text:   prefix + err,
		})
	}
	return checks
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
