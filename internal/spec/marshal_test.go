package spec

import (
	"strings"
	"testing"
)

func TestMarshalOmitsUnusedSections(t *testing.T) {
	doc := &Spec{
		Name: "dns-only",
		Steps: []Step{
			{
				Name: "lookup",
				Kind: KindDNS,
				DNS: DNSQuery{
					Name: "example.org",
				},
			},
		},
	}

	data, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal() err = %v", err)
	}

	text := string(data)
	if !strings.Contains(text, "dns:") {
		t.Fatalf("marshal output missing dns block:\n%s", text)
	}
	if strings.Contains(text, "request:") {
		t.Fatalf("marshal output should omit request block:\n%s", text)
	}
	if strings.Contains(text, "tls:") || strings.Contains(text, "tcp:") {
		t.Fatalf("marshal output should omit unrelated transport blocks:\n%s", text)
	}
}

func TestMarshalPreservesHTTPPathWhenBaseURLIsSet(t *testing.T) {
	doc := &Spec{
		BaseURL: "https://example.org",
		Steps: []Step{
			{
				Name: "health",
				Request: Request{
					Method: "GET",
					Path:   "/health",
				},
			},
		},
	}

	data, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal() err = %v", err)
	}

	text := string(data)
	if !strings.Contains(text, "path: /health") {
		t.Fatalf("marshal output missing request.path:\n%s", text)
	}
	if strings.Contains(text, "url: https://example.org/health") {
		t.Fatalf("marshal output should preserve request.path form:\n%s", text)
	}
}
