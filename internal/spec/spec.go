package spec

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Spec struct {
	Name    string `yaml:"name"`
	BaseURL string `yaml:"base_url"`
	Steps   []Step `yaml:"steps"`
}

type Step struct {
	Name    string  `yaml:"name"`
	Request Request `yaml:"request"`
	Expect  Expect  `yaml:"expect"`
}

type Request struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
}

type Expect struct {
	Status       int      `yaml:"status"`
	BodyContains []string `yaml:"body_contains"`
	BodyAbsent   []string `yaml:"body_absent"`
	HeaderHas    []string `yaml:"header_contains"`
}

func Load(path string) (*Spec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var s Spec
	if err := yaml.Unmarshal(b, &s); err != nil {
		return nil, err
	}

	if len(s.Steps) == 0 {
		return nil, fmt.Errorf("spec has no steps")
	}

	var base *url.URL
	if strings.TrimSpace(s.BaseURL) != "" {
		parsed, err := url.Parse(s.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid base_url: %w", err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("base_url must include scheme and host")
		}
		base = parsed
	}

	for i, step := range s.Steps {
		if step.Name == "" {
			s.Steps[i].Name = fmt.Sprintf("step-%d", i+1)
		}
		if s.Steps[i].Request.Method == "" {
			s.Steps[i].Request.Method = "GET"
		} else {
			s.Steps[i].Request.Method = strings.ToUpper(s.Steps[i].Request.Method)
		}

		requestURL := strings.TrimSpace(s.Steps[i].Request.URL)
		requestPath := strings.TrimSpace(s.Steps[i].Request.Path)
		switch {
		case requestURL != "" && requestPath != "":
			return nil, fmt.Errorf("%s: use request.url or request.path, not both", s.Steps[i].Name)
		case requestURL == "" && requestPath == "":
			return nil, fmt.Errorf("%s: request.url or request.path is required", s.Steps[i].Name)
		case requestURL == "" && requestPath != "":
			if base == nil {
				return nil, fmt.Errorf("%s: request.path requires base_url", s.Steps[i].Name)
			}
			relative, err := url.Parse(requestPath)
			if err != nil {
				return nil, fmt.Errorf("%s: invalid request.path: %w", s.Steps[i].Name, err)
			}
			s.Steps[i].Request.URL = base.ResolveReference(relative).String()
		}
		if strings.TrimSpace(s.Steps[i].Request.URL) == "" {
			return nil, fmt.Errorf("%s: request.url is required", s.Steps[i].Name)
		}
	}

	return &s, nil
}
