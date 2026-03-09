package adhoc

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/nickromney/website-testing/internal/spec"
)

type HTTPOptions struct {
	Method         string
	Status         int
	Body           string
	Headers        map[string]string
	BodyContains   []string
	BodyAbsent     []string
	HeaderContains []string
}

func HTTPCheck(target string, opts HTTPOptions) (spec.Step, error) {
	normalized, err := NormalizeTarget(target)
	if err != nil {
		return spec.Step{}, err
	}

	method := strings.ToUpper(strings.TrimSpace(opts.Method))
	if method == "" {
		method = "GET"
	}

	status := opts.Status
	if status == 0 {
		status = 200
	}

	step := spec.Step{
		Name: "quick smoke " + normalized,
		Kind: spec.KindHTTP,
		Request: spec.Request{
			Method:  method,
			URL:     normalized,
			Headers: cloneHeaders(opts.Headers),
			Body:    opts.Body,
		},
		Expect: spec.Expect{
			Status:       status,
			BodyContains: append([]string(nil), opts.BodyContains...),
			BodyAbsent:   append([]string(nil), opts.BodyAbsent...),
			HeaderHas:    append([]string(nil), opts.HeaderContains...),
		},
	}

	return step, nil
}

func NormalizeTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("target is required")
	}

	if strings.Contains(target, "://") {
		parsed, err := url.Parse(target)
		if err != nil {
			return "", fmt.Errorf("invalid target %q: %w", target, err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return "", fmt.Errorf("target %q must include scheme and host", target)
		}
		return parsed.String(), nil
	}

	if strings.HasPrefix(target, "/") {
		return "", fmt.Errorf("quick smoke target must be a host or URL, got path %q", target)
	}

	if strings.Contains(target, "/") || strings.Contains(target, "?") || strings.Contains(target, "#") {
		parsed, err := url.Parse("https://" + target)
		if err != nil {
			return "", fmt.Errorf("invalid target %q: %w", target, err)
		}
		if parsed.Host == "" {
			return "", fmt.Errorf("target %q must include a host", target)
		}
		return parsed.String(), nil
	}

	if strings.Contains(target, ":") {
		if _, _, err := net.SplitHostPort(target); err != nil {
			return "", fmt.Errorf("invalid target %q: %w", target, err)
		}
	}

	return "https://" + target, nil
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = v
	}
	return out
}
