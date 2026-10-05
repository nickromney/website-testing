package runner

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nickromney/website-testing/internal/spec"
)

type offlineTransport func(*http.Request) (*http.Response, error)

func (f offlineTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestOfflineTransportContract(t *testing.T) {
	r, err := New(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	r.client.Transport = offlineTransport(func(req *http.Request) (*http.Response, error) {
		if req.Method != "POST" || req.Header.Get("X-Fixture") != "synthetic" {
			t.Fatal("request contract lost")
		}
		body, err := io.ReadAll(req.Body)
		if err != nil || string(body) != "fixture" {
			t.Fatalf("body %q: %v", body, err)
		}
		return &http.Response{StatusCode: 201, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Request: req}, nil
	})
	res := r.RunStep(context.Background(), spec.Step{Kind: spec.KindHTTP, Request: spec.Request{Method: "POST", URL: "https://synthetic.invalid/", Body: "fixture", Headers: map[string]string{"X-Fixture": "synthetic"}}, Expect: spec.Expect{Status: 201, BodyContains: []string{`"ok":true`}}})
	if !res.Passed {
		t.Fatalf("offline result: %+v", res)
	}
}

func TestOfflineCancellationAndDeadline(t *testing.T) {
	for _, cancelNow := range []bool{true, false} {
		t.Run(map[bool]string{true: "cancelled", false: "deadline"}[cancelNow], func(t *testing.T) {
			r, err := New(time.Second)
			if err != nil {
				t.Fatal(err)
			}
			r.client.Transport = offlineTransport(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				return nil, req.Context().Err()
			})
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if cancelNow {
				cancel()
			}
			started := time.Now()
			res := r.RunStep(ctx, spec.Step{Kind: spec.KindHTTP, Request: spec.Request{Method: "GET", URL: "https://synthetic.invalid/"}})
			if res.Passed || len(res.Errors) == 0 {
				t.Fatalf("expected context failure: %+v", res)
			}
			wanted := "deadline exceeded"
			if cancelNow {
				wanted = "context canceled"
			}
			if !strings.Contains(strings.Join(res.Errors, " "), wanted) {
				t.Fatalf("wrong failure: %v", res.Errors)
			}
			if time.Since(started) > time.Second {
				t.Fatal("cancellation was not bounded")
			}
		})
	}
}
