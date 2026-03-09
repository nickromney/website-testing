package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
