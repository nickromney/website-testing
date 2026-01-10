package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.yaml")
	if err := os.WriteFile(p, []byte("steps:\n  - name: one\n    request:\n      url: https://example.test/\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}
	if len(s.Steps) != 1 {
		t.Fatalf("steps = %d", len(s.Steps))
	}
	if s.Steps[0].Request.Method != "GET" {
		t.Fatalf("method = %q", s.Steps[0].Request.Method)
	}
}
