package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHumanHTTPDoesNotCertifyIncompleteBody(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "smoke")
	build := exec.Command("go", "build", "-o", binary, "./cmd/smoke-go")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build public CLI: %v\n%s", err, output)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if r.URL.Path == "/broken" {
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, "known")
			return
		}
		if r.URL.Path == "/oversize" {
			w.(http.Flusher).Flush() // Exercise a response without Content-Length.
			_, _ = io.WriteString(w, "known"+strings.Repeat("x", 1024*1024)+"forbidden")
			return
		}
		_, _ = io.WriteString(w, "known complete response")
	}))
	t.Cleanup(server.Close)
	for _, path := range []string{"/oversize", "/broken", "/good"} {
		t.Run(path, func(t *testing.T) {
			output, err := exec.Command(binary, server.URL+path, "--body-contains", "known", "--body-absent", "forbidden", "--header-contains", "Content-Type: text/plain").CombinedOutput()
			text := string(output)
			if !strings.Contains(text, "[ OK ] 200 Response code") || !strings.Contains(text, `[ OK ] Headers contain "Content-Type: text/plain"`) {
				t.Fatalf("observed status/header facts were lost: %s", text)
			}
			if path == "/good" {
				if err != nil || !strings.Contains(text, `[ OK ] Body contains "known"`) || !strings.Contains(text, `[ OK ] (Assert absence of string): Body does not contain "forbidden"`) {
					t.Fatalf("complete response changed: %v\n%s", err, text)
				}
				return
			}
			if err == nil || !strings.Contains(text, "Body assertions not evaluated: response capture is incomplete") || strings.Contains(text, `[ OK ] Body contains`) || strings.Contains(text, `[ OK ] (Assert absence of string): Body`) {
				t.Fatalf("incomplete human response falsely certified: %v\n%s", err, text)
			}
		})
	}
}
