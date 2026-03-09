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
	Name    string `yaml:"name"`
	BaseURL string `yaml:"base_url"`
	Steps   []Step `yaml:"steps"`
}

type Step struct {
	Name    string   `yaml:"name"`
	Kind    string   `yaml:"kind"`
	Request Request  `yaml:"request"`
	DNS     DNSQuery `yaml:"dns"`
	TLS     TLSProbe `yaml:"tls"`
	TCP     TCPProbe `yaml:"tcp"`
	Expect  Expect   `yaml:"expect"`
}

type Request struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
}

type DNSQuery struct {
	Name   string `yaml:"name"`
	Type   string `yaml:"type"`
	Server string `yaml:"server"`
}

type TLSProbe struct {
	Address    string `yaml:"address"`
	ServerName string `yaml:"server_name"`
}

type TCPProbe struct {
	Address string `yaml:"address"`
}

type Expect struct {
	Status               int      `yaml:"status"`
	BodyContains         []string `yaml:"body_contains"`
	BodyAbsent           []string `yaml:"body_absent"`
	HeaderHas            []string `yaml:"header_contains"`
	AnswerContains       []string `yaml:"answer_contains"`
	AnswerAbsent         []string `yaml:"answer_absent"`
	DaysRemainingAtLeast *int     `yaml:"days_remaining_at_least"`
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
		if strings.TrimSpace(step.Name) == "" {
			s.Steps[i].Name = fmt.Sprintf("step-%d", i+1)
		}

		s.Steps[i].Kind = normalizeKind(step.Kind)
		switch s.Steps[i].Kind {
		case KindHTTP:
			if err := validateHTTP(base, &s.Steps[i]); err != nil {
				return nil, err
			}
		case KindDNS:
			if err := validateDNS(&s.Steps[i]); err != nil {
				return nil, err
			}
		case KindTLS:
			if err := validateTLS(&s.Steps[i]); err != nil {
				return nil, err
			}
		case KindTCP:
			if err := validateTCP(&s.Steps[i]); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s: unsupported kind %q", s.Steps[i].Name, s.Steps[i].Kind)
		}
	}

	return &s, nil
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
