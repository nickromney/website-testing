package spec

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	KindHTTP = "http"
	KindDNS  = "dns"
	KindTLS  = "tls"
	KindTCP  = "tcp"
)

type Spec struct {
	Name    string `yaml:"name,omitempty"`
	BaseURL string `yaml:"base_url,omitempty"`
	Steps   []Step `yaml:"steps"`
}

type Step struct {
	Name    string   `yaml:"name,omitempty"`
	Kind    string   `yaml:"kind,omitempty"`
	Request Request  `yaml:"request,omitempty"`
	DNS     DNSQuery `yaml:"dns,omitempty"`
	TLS     TLSProbe `yaml:"tls,omitempty"`
	TCP     TCPProbe `yaml:"tcp,omitempty"`
	Expect  Expect   `yaml:"expect,omitempty"`
}

type Request struct {
	Method  string            `yaml:"method,omitempty"`
	URL     string            `yaml:"url,omitempty"`
	Path    string            `yaml:"path,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
}

type DNSQuery struct {
	Name   string `yaml:"name,omitempty"`
	Type   string `yaml:"type,omitempty"`
	Server string `yaml:"server,omitempty"`
}

type TLSProbe struct {
	Address    string `yaml:"address,omitempty"`
	ServerName string `yaml:"server_name,omitempty"`
}

type TCPProbe struct {
	Address string `yaml:"address,omitempty"`
}

type Expect struct {
	Status               int      `yaml:"status,omitempty"`
	BodyContains         []string `yaml:"body_contains,omitempty"`
	BodyAbsent           []string `yaml:"body_absent,omitempty"`
	HeaderHas            []string `yaml:"header_contains,omitempty"`
	AnswerContains       []string `yaml:"answer_contains,omitempty"`
	AnswerAbsent         []string `yaml:"answer_absent,omitempty"`
	DaysRemainingAtLeast *int     `yaml:"days_remaining_at_least,omitempty"`
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

	if err := Normalise(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

func Normalise(s *Spec) error {
	if s == nil {
		return fmt.Errorf("spec is nil")
	}

	if len(s.Steps) == 0 {
		return fmt.Errorf("spec has no steps")
	}

	s.Name = strings.TrimSpace(s.Name)
	s.BaseURL = strings.TrimSpace(s.BaseURL)

	var base *url.URL
	if strings.TrimSpace(s.BaseURL) != "" {
		parsed, err := url.Parse(s.BaseURL)
		if err != nil {
			return fmt.Errorf("invalid base_url: %w", err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("base_url must include scheme and host")
		}
		base = parsed
	}

	for i, step := range s.Steps {
		if strings.TrimSpace(step.Name) == "" {
			s.Steps[i].Name = fmt.Sprintf("step-%d", i+1)
		}

		s.Steps[i].Kind = normalizeKind(step.Kind)
		switch s.Steps[i].Kind {
		case KindHTTP:
			if err := validateHTTP(base, &s.Steps[i]); err != nil {
				return err
			}
		case KindDNS:
			if err := validateDNS(&s.Steps[i]); err != nil {
				return err
			}
		case KindTLS:
			if err := validateTLS(&s.Steps[i]); err != nil {
				return err
			}
		case KindTCP:
			if err := validateTCP(&s.Steps[i]); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: unsupported kind %q", s.Steps[i].Name, s.Steps[i].Kind)
		}
	}

	return nil
}

func Marshal(s *Spec) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("spec is nil")
	}

	clone := cloneSpec(s)
	for i := range clone.Steps {
		if strings.TrimSpace(clone.Steps[i].Request.Path) != "" {
			clone.Steps[i].Request.URL = ""
		}
	}

	if err := Normalise(clone); err != nil {
		return nil, err
	}

	type yamlStep struct {
		Name    string    `yaml:"name,omitempty"`
		Kind    string    `yaml:"kind"`
		Request *Request  `yaml:"request,omitempty"`
		DNS     *DNSQuery `yaml:"dns,omitempty"`
		TLS     *TLSProbe `yaml:"tls,omitempty"`
		TCP     *TCPProbe `yaml:"tcp,omitempty"`
		Expect  *Expect   `yaml:"expect,omitempty"`
	}

	type yamlSpec struct {
		Name    string     `yaml:"name,omitempty"`
		BaseURL string     `yaml:"base_url,omitempty"`
		Steps   []yamlStep `yaml:"steps"`
	}

	out := yamlSpec{
		Name:    clone.Name,
		BaseURL: clone.BaseURL,
		Steps:   make([]yamlStep, 0, len(clone.Steps)),
	}

	for i, step := range clone.Steps {
		yamlStep := yamlStep{
			Name: step.Name,
			Kind: normalizeKind(step.Kind),
		}

		switch yamlStep.Kind {
		case KindDNS:
			dns := step.DNS
			yamlStep.DNS = &dns
		case KindTLS:
			tls := step.TLS
			yamlStep.TLS = &tls
		case KindTCP:
			tcp := step.TCP
			yamlStep.TCP = &tcp
		default:
			req := step.Request
			if strings.TrimSpace(s.Steps[i].Request.Path) != "" {
				req.URL = ""
				req.Path = strings.TrimSpace(s.Steps[i].Request.Path)
			} else {
				req.Path = ""
			}
			yamlStep.Request = &req
		}

		if hasExpect(step.Expect) {
			expect := step.Expect
			yamlStep.Expect = &expect
		}

		out.Steps = append(out.Steps, yamlStep)
	}

	return yaml.Marshal(&out)
}

func normalizeKind(kind string) string {
	kind = strings.TrimSpace(strings.ToLower(kind))
	if kind == "" {
		return KindHTTP
	}
	return kind
}

func validateHTTP(base *url.URL, step *Step) error {
	if step.Request.Method == "" {
		step.Request.Method = "GET"
	} else {
		step.Request.Method = strings.ToUpper(step.Request.Method)
	}

	requestURL := strings.TrimSpace(step.Request.URL)
	requestPath := strings.TrimSpace(step.Request.Path)
	switch {
	case requestURL != "" && requestPath != "":
		return fmt.Errorf("%s: use request.url or request.path, not both", step.Name)
	case requestURL == "" && requestPath == "":
		return fmt.Errorf("%s: request.url or request.path is required", step.Name)
	case requestURL == "" && requestPath != "":
		if base == nil {
			return fmt.Errorf("%s: request.path requires base_url", step.Name)
		}
		relative, err := url.Parse(requestPath)
		if err != nil {
			return fmt.Errorf("%s: invalid request.path: %w", step.Name, err)
		}
		step.Request.URL = base.ResolveReference(relative).String()
	}

	if strings.TrimSpace(step.Request.URL) == "" {
		return fmt.Errorf("%s: request.url is required", step.Name)
	}

	return nil
}

func validateDNS(step *Step) error {
	if strings.TrimSpace(step.DNS.Name) == "" {
		return fmt.Errorf("%s: dns.name is required", step.Name)
	}

	if step.DNS.Type == "" {
		step.DNS.Type = "A"
	} else {
		step.DNS.Type = strings.ToUpper(strings.TrimSpace(step.DNS.Type))
	}

	switch step.DNS.Type {
	case "A", "AAAA", "CNAME", "MX", "NS", "TXT":
	default:
		return fmt.Errorf("%s: unsupported dns.type %q", step.Name, step.DNS.Type)
	}

	if strings.TrimSpace(step.DNS.Server) != "" {
		server, err := normalizeAddress(step.DNS.Server, "53")
		if err != nil {
			return fmt.Errorf("%s: invalid dns.server: %w", step.Name, err)
		}
		step.DNS.Server = server
	}

	return nil
}

func validateTLS(step *Step) error {
	if strings.TrimSpace(step.TLS.Address) == "" {
		return fmt.Errorf("%s: tls.address is required", step.Name)
	}

	address, err := normalizeAddress(step.TLS.Address, "443")
	if err != nil {
		return fmt.Errorf("%s: invalid tls.address: %w", step.Name, err)
	}
	step.TLS.Address = address

	if strings.TrimSpace(step.TLS.ServerName) == "" {
		host, _, err := net.SplitHostPort(step.TLS.Address)
		if err != nil {
			return fmt.Errorf("%s: invalid tls.address: %w", step.Name, err)
		}
		step.TLS.ServerName = host
	}

	return nil
}

func validateTCP(step *Step) error {
	if strings.TrimSpace(step.TCP.Address) == "" {
		return fmt.Errorf("%s: tcp.address is required", step.Name)
	}

	address, err := normalizeAddress(step.TCP.Address, "80")
	if err != nil {
		return fmt.Errorf("%s: invalid tcp.address: %w", step.Name, err)
	}
	step.TCP.Address = address

	return nil
}

func normalizeAddress(address string, defaultPort string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", fmt.Errorf("address is required")
	}
	if strings.Contains(address, "://") {
		return "", fmt.Errorf("use host[:port], not URL")
	}

	host := address
	if parsedHost, parsedPort, err := net.SplitHostPort(address); err == nil {
		if parsedHost == "" {
			return "", fmt.Errorf("host is required")
		}
		if parsedPort == "" {
			return "", fmt.Errorf("port is required")
		}
		return net.JoinHostPort(parsedHost, parsedPort), nil
	}

	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	}
	if host == "" {
		return "", fmt.Errorf("host is required")
	}

	return net.JoinHostPort(host, defaultPort), nil
}

func hasExpect(expect Expect) bool {
	return expect.Status != 0 ||
		len(expect.BodyContains) > 0 ||
		len(expect.BodyAbsent) > 0 ||
		len(expect.HeaderHas) > 0 ||
		len(expect.AnswerContains) > 0 ||
		len(expect.AnswerAbsent) > 0 ||
		expect.DaysRemainingAtLeast != nil
}

func cloneSpec(s *Spec) *Spec {
	if s == nil {
		return nil
	}

	clone := &Spec{
		Name:    s.Name,
		BaseURL: s.BaseURL,
		Steps:   make([]Step, len(s.Steps)),
	}

	for i, step := range s.Steps {
		clone.Steps[i] = Step{
			Name: step.Name,
			Kind: step.Kind,
			Request: Request{
				Method: step.Request.Method,
				URL:    step.Request.URL,
				Path:   step.Request.Path,
				Body:   step.Request.Body,
			},
			DNS: step.DNS,
			TLS: step.TLS,
			TCP: step.TCP,
			Expect: Expect{
				Status:               step.Expect.Status,
				DaysRemainingAtLeast: step.Expect.DaysRemainingAtLeast,
			},
		}

		if len(step.Request.Headers) > 0 {
			clone.Steps[i].Request.Headers = make(map[string]string, len(step.Request.Headers))
			for k, v := range step.Request.Headers {
				clone.Steps[i].Request.Headers[k] = v
			}
		}

		clone.Steps[i].Expect.BodyContains = append([]string(nil), step.Expect.BodyContains...)
		clone.Steps[i].Expect.BodyAbsent = append([]string(nil), step.Expect.BodyAbsent...)
		clone.Steps[i].Expect.HeaderHas = append([]string(nil), step.Expect.HeaderHas...)
		clone.Steps[i].Expect.AnswerContains = append([]string(nil), step.Expect.AnswerContains...)
		clone.Steps[i].Expect.AnswerAbsent = append([]string(nil), step.Expect.AnswerAbsent...)
		if step.Expect.DaysRemainingAtLeast != nil {
			days := *step.Expect.DaysRemainingAtLeast
			clone.Steps[i].Expect.DaysRemainingAtLeast = &days
		}
	}

	return clone
}
