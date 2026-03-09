package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHTTPDefaults(t *testing.T) {
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
	if s.Steps[0].Kind != KindHTTP {
		t.Fatalf("kind = %q", s.Steps[0].Kind)
	}
	if s.Steps[0].Request.Method != "GET" {
		t.Fatalf("method = %q", s.Steps[0].Request.Method)
	}
}

func TestLoadResolvesPathAgainstBaseURL(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.yaml")
	content := `name: Example
base_url: https://example.test
steps:
  - name: home
    request:
      path: /health
      method: post
`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}
	if got := s.Steps[0].Request.URL; got != "https://example.test/health" {
		t.Fatalf("url = %q", got)
	}
	if got := s.Steps[0].Request.Method; got != "POST" {
		t.Fatalf("method = %q", got)
	}
}

func TestLoadSupportsDNSAndNetworkSteps(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.yaml")
	content := `steps:
  - name: dns
    kind: dns
    dns:
      name: example.test
      type: txt
      server: 127.0.0.1
  - name: tls
    kind: tls
    tls:
      address: example.test
  - name: tcp
    kind: tcp
    tcp:
      address: localhost:8080
`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	if got := s.Steps[0].DNS.Type; got != "TXT" {
		t.Fatalf("dns.type = %q", got)
	}
	if got := s.Steps[0].DNS.Server; got != "127.0.0.1:53" {
		t.Fatalf("dns.server = %q", got)
	}
	if got := s.Steps[1].TLS.Address; got != "example.test:443" {
		t.Fatalf("tls.address = %q", got)
	}
	if got := s.Steps[2].TCP.Address; got != "localhost:8080" {
		t.Fatalf("tcp.address = %q", got)
	}
}

func TestLegacyParityExamplesLoad(t *testing.T) {
	examplesDir := filepath.Join("..", "..", "examples")

	tests := []struct {
		name      string
		stepCount int
		kinds     []string
	}{
		{name: "smoke-google.yaml", stepCount: 1, kinds: []string{KindHTTP}},
		{name: "smoke-theregister.yaml", stepCount: 3, kinds: []string{KindHTTP, KindHTTP, KindHTTP}},
		{name: "smoke-dig.yaml", stepCount: 2, kinds: []string{KindDNS, KindDNS}},
		{name: "smoke-ssl.yaml", stepCount: 1, kinds: []string{KindTLS}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(examplesDir, tt.name)
			s, err := Load(path)
			if err != nil {
				t.Fatalf("Load(%q) err = %v", path, err)
			}
			if len(s.Steps) != tt.stepCount {
				t.Fatalf("steps = %d, want %d", len(s.Steps), tt.stepCount)
			}
			for i, wantKind := range tt.kinds {
				if got := s.Steps[i].Kind; got != wantKind {
					t.Fatalf("step %d kind = %q, want %q", i, got, wantKind)
				}
			}
		})
	}
}
