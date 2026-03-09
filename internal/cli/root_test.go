package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommandJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello smoke-go"))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.yaml")
	specBody := "steps:\n  - name: hello\n    request:\n      url: " + srv.URL + "\n    expect:\n      status: 200\n      body_contains:\n        - hello\n"
	if err := os.WriteFile(specPath, []byte(specBody), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd(BuildInfo{Version: "test"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"run", specPath, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() err = %v", err)
	}

	var out runOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("json.Unmarshal() err = %v, output=%q", err, stdout.String())
	}
	if !out.Passed {
		t.Fatalf("expected passed output, got %#v", out)
	}
	if len(out.Results) != 1 {
		t.Fatalf("results = %d", len(out.Results))
	}
	if got := out.Results[0].StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d", got)
	}
}

func TestRootQuickTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello quick smoke"))
	}))
	t.Cleanup(srv.Close)

	cmd := NewRootCmd(BuildInfo{Version: "test"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{srv.URL, "--body-contains", "hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() err = %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "> "+srv.URL) {
		t.Fatalf("expected target heading, got %q", out)
	}
	if !strings.Contains(out, "[ OK ] 200 Response code") {
		t.Fatalf("expected status assertion, got %q", out)
	}
	if !strings.Contains(out, `[ OK ] Body contains "hello"`) {
		t.Fatalf("expected body assertion, got %q", out)
	}
	if !strings.Contains(out, "OK (2/2)") {
		t.Fatalf("expected success summary, got %q", out)
	}
}

func TestRootQuickTargetAssertionFailureSeparatesHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello quick smoke"))
	}))
	t.Cleanup(srv.Close)

	cmd := NewRootCmd(BuildInfo{Version: "test"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{srv.URL, "--body-contains", "bananas"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected Execute() to fail")
	}
	if got := err.Error(); got != "one or more steps failed" {
		t.Fatalf("unexpected error %q", got)
	}

	out := stdout.String()
	if !strings.Contains(out, "[ OK ] 200 Response code") {
		t.Fatalf("expected HTTP success line, got %q", out)
	}
	if !strings.Contains(out, `[FAIL] Body does not contain "bananas"`) {
		t.Fatalf("expected assertion failure line, got %q", out)
	}
	if !strings.Contains(out, "FAIL (1/2)") {
		t.Fatalf("expected failure summary, got %q", out)
	}
}

func TestRunCommandLegacyStyleSummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("home"))
		case "/security":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("security news"))
		case "/2022/08/18/non-existent":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("Sorry, this page doesn't exist!"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.yaml")
	specBody := "base_url: " + srv.URL + "\nsteps:\n" +
		"  - name: home\n    request:\n      path: /\n    expect:\n      status: 200\n" +
		"  - name: security\n    request:\n      path: /security\n    expect:\n      status: 200\n      body_contains:\n        - Oh no, you're thinking, yet another cookie pop-up.\n      body_absent:\n        - Sorry, this page doesn't exist!\n" +
		"  - name: missing\n    request:\n      path: /2022/08/18/non-existent\n    expect:\n      status: 404\n      body_contains:\n        - Sorry, this page doesn't exist!\n"
	if err := os.WriteFile(specPath, []byte(specBody), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd(BuildInfo{Version: "test"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"run", specPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected Execute() to fail")
	}
	if got := err.Error(); got != "one or more steps failed" {
		t.Fatalf("unexpected error %q", got)
	}

	out := stdout.String()
	if !strings.Contains(out, "[ OK ] 200 Response code") {
		t.Fatalf("expected 200 status assertion, got %q", out)
	}
	if !strings.Contains(out, `[FAIL] Body does not contain "Oh no, you're thinking, yet another cookie pop-up."`) {
		t.Fatalf("expected failed cookie assertion, got %q", out)
	}
	if !strings.Contains(out, `[ OK ] (Assert absence of string): Body does not contain "Sorry, this page doesn't exist!"`) {
		t.Fatalf("expected absence assertion, got %q", out)
	}
	if !strings.Contains(out, "[ OK ] 404 Response code") {
		t.Fatalf("expected 404 status assertion, got %q", out)
	}
	if !strings.Contains(out, "FAIL (1/6)") {
		t.Fatalf("expected legacy-style summary, got %q", out)
	}
}

func TestRootQuickTargetUsesColorWhenForced(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello quick smoke"))
	}))
	t.Cleanup(srv.Close)

	cmd := NewRootCmd(BuildInfo{Version: "test"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{srv.URL, "--body-contains", "hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() err = %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "[ \x1b[1;32mOK\x1b[0m ]") {
		t.Fatalf("expected colored OK label, got %q", out)
	}
	if !strings.Contains(out, "\x1b[42mOK (2/2)\x1b[0m") {
		t.Fatalf("expected green summary background, got %q", out)
	}
}

func TestRootQuickTargetNoColorDisablesAutoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello quick smoke"))
	}))
	t.Cleanup(srv.Close)

	cmd := NewRootCmd(BuildInfo{Version: "test"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{srv.URL, "--body-contains", "hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() err = %v", err)
	}

	out := stdout.String()
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("expected plain output when NO_COLOR is set, got %q", out)
	}
}
