package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/nickromney/website-testing/internal/spec"
)

type Result struct {
	Step       spec.Step
	StartedAt  time.Time
	Duration   time.Duration
	StatusCode int
	Headers    http.Header
	Body       []byte
	Passed     bool
	Errors     []string
}

type Runner struct {
	client *http.Client
}

func New(timeout time.Duration) (*Runner, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	return &Runner{client: &http.Client{Timeout: timeout, Jar: jar}}, nil
}

func (r *Runner) RunStep(ctx context.Context, step spec.Step) Result {
	started := time.Now()
	res := Result{Step: step, StartedAt: started}

	reqBody := io.Reader(nil)
	if step.Request.Body != "" {
		reqBody = strings.NewReader(step.Request.Body)
	}

	req, err := http.NewRequestWithContext(ctx, step.Request.Method, step.Request.URL, reqBody)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		res.Duration = time.Since(started)
		return res
	}

	for k, v := range step.Request.Headers {
		req.Header.Set(k, v)
	}

	httpRes, err := r.client.Do(req)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		res.Duration = time.Since(started)
		return res
	}
	defer httpRes.Body.Close()

	res.StatusCode = httpRes.StatusCode
	res.Headers = httpRes.Header.Clone()

	body, _ := io.ReadAll(io.LimitReader(httpRes.Body, 1024*1024))
	res.Body = body
	res.Duration = time.Since(started)

	res.Passed = evaluate(step.Expect, httpRes.StatusCode, httpRes.Header, body, &res.Errors)
	return res
}

func evaluate(expect spec.Expect, status int, headers http.Header, body []byte, errs *[]string) bool {
	ok := true
	if expect.Status != 0 && status != expect.Status {
		ok = false
		*errs = append(*errs, fmt.Sprintf("expected status %d, got %d", expect.Status, status))
	}

	bodyStr := string(body)
	for _, s := range expect.BodyContains {
		if s == "" {
			continue
		}
		if !strings.Contains(bodyStr, s) {
			ok = false
			*errs = append(*errs, fmt.Sprintf("expected body to contain %q", s))
		}
	}

	for _, s := range expect.BodyAbsent {
		if s == "" {
			continue
		}
		if strings.Contains(bodyStr, s) {
			ok = false
			*errs = append(*errs, fmt.Sprintf("expected body to NOT contain %q", s))
		}
	}

	if len(expect.HeaderHas) != 0 {
		var buf bytes.Buffer
		for k, vs := range headers {
			for _, v := range vs {
				buf.WriteString(k)
				buf.WriteString(": ")
				buf.WriteString(v)
				buf.WriteByte('\n')
			}
		}
		headerText := buf.String()
		for _, s := range expect.HeaderHas {
			if s == "" {
				continue
			}
			if !strings.Contains(headerText, s) {
				ok = false
				*errs = append(*errs, fmt.Sprintf("expected headers to contain %q", s))
			}
		}
	}

	return ok
}
