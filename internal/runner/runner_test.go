package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nickromney/website-testing/internal/spec"
)

func TestRunStep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("hello world"))
	}))
	t.Cleanup(srv.Close)

	r, err := New(2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}

	step := spec.Step{
		Name: "one",
		Request: spec.Request{
			Method: "GET",
			URL:    srv.URL,
		},
		Expect: spec.Expect{
			Status:       200,
			BodyContains: []string{"hello"},
			BodyAbsent:   []string{"nope"},
			HeaderHas:    []string{"Content-Type: text/plain"},
		},
	}

	res := r.RunStep(context.Background(), step)
	if !res.Passed {
		t.Fatalf("expected pass, errors=%v", res.Errors)
	}
}
