package spec

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Spec struct {
	Steps []Step `yaml:"steps"`
}

type Step struct {
	Name    string  `yaml:"name"`
	Request Request `yaml:"request"`
	Expect  Expect  `yaml:"expect"`
}

type Request struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
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

	for i, step := range s.Steps {
		if step.Name == "" {
			s.Steps[i].Name = fmt.Sprintf("step-%d", i+1)
		}
		if s.Steps[i].Request.Method == "" {
			s.Steps[i].Request.Method = "GET"
		}
		if s.Steps[i].Request.URL == "" {
			return nil, fmt.Errorf("%s: request.url is required", s.Steps[i].Name)
		}
	}

	return &s, nil
}
