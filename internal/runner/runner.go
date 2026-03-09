package runner

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"sort"
	"strings"
	"time"

	"github.com/nickromney/website-testing/internal/spec"
)

type Result struct {
	Step      spec.Step
	StartedAt time.Time
	Duration  time.Duration
	Passed    bool
	Errors    []string
	Kind      string

	StatusCode int
	Headers    http.Header
	Body       []byte

	DNSName       string
	DNSRecordType string
	DNSResolver   string
	DNSAnswers    []string

	TLSAddress       string
	TLSServerName    string
	TLSSubject       string
	TLSIssuer        string
	TLSNotAfter      time.Time
	TLSDaysRemaining int

	TCPAddress string
}

type Runner struct {
	client  *http.Client
	timeout time.Duration
	dialer  net.Dialer
}

func New(timeout time.Duration) (*Runner, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	dialer := net.Dialer{Timeout: timeout}
	return &Runner{
		client:  &http.Client{Timeout: timeout, Jar: jar},
		timeout: timeout,
		dialer:  dialer,
	}, nil
}

func (r *Runner) RunStep(ctx context.Context, step spec.Step) Result {
	switch step.Kind {
	case spec.KindDNS:
		return r.runDNSStep(ctx, step)
	case spec.KindTLS:
		return r.runTLSStep(ctx, step)
	case spec.KindTCP:
		return r.runTCPStep(ctx, step)
	default:
		return r.runHTTPStep(ctx, step)
	}
}

func (r *Runner) runHTTPStep(ctx context.Context, step spec.Step) Result {
	started := time.Now()
	res := Result{Step: step, StartedAt: started, Kind: spec.KindHTTP}

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

	body, readErr := io.ReadAll(io.LimitReader(httpRes.Body, 1024*1024))
	if readErr != nil {
		res.Errors = append(res.Errors, readErr.Error())
	}
	res.Body = body
	res.Duration = time.Since(started)

	res.Passed = evaluateHTTP(step.Expect, httpRes.StatusCode, httpRes.Header, body, &res.Errors) && len(res.Errors) == 0
	return res
}

func (r *Runner) runDNSStep(ctx context.Context, step spec.Step) Result {
	started := time.Now()
	res := Result{
		Step:          step,
		StartedAt:     started,
		Kind:          spec.KindDNS,
		DNSName:       step.DNS.Name,
		DNSRecordType: step.DNS.Type,
		DNSResolver:   step.DNS.Server,
	}

	answers, err := r.lookupDNS(ctx, step.DNS)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		res.Duration = time.Since(started)
		return res
	}

	res.DNSAnswers = answers
	res.Body = []byte(strings.Join(answers, "\n"))
	res.Duration = time.Since(started)
	res.Passed = evaluateDNS(step.Expect, answers, res.Body, &res.Errors) && len(res.Errors) == 0
	return res
}

func (r *Runner) runTLSStep(ctx context.Context, step spec.Step) Result {
	started := time.Now()
	res := Result{
		Step:          step,
		StartedAt:     started,
		Kind:          spec.KindTLS,
		TLSAddress:    step.TLS.Address,
		TLSServerName: step.TLS.ServerName,
	}

	tlsDialer := tls.Dialer{
		NetDialer: &r.dialer,
		Config: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         step.TLS.ServerName,
		},
	}

	conn, err := tlsDialer.DialContext(ctx, "tcp", step.TLS.Address)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		res.Duration = time.Since(started)
		return res
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		res.Errors = append(res.Errors, "unexpected non-TLS connection")
		res.Duration = time.Since(started)
		return res
	}

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		res.Errors = append(res.Errors, "no peer certificates presented")
		res.Duration = time.Since(started)
		return res
	}

	cert := state.PeerCertificates[0]
	res.TLSSubject = cert.Subject.String()
	res.TLSIssuer = cert.Issuer.String()
	res.TLSNotAfter = cert.NotAfter
	res.TLSDaysRemaining = int(time.Until(cert.NotAfter).Hours() / 24)
	res.Body = []byte(fmt.Sprintf(
		"subject: %s\nissuer: %s\nnot_after: %s\ndays_remaining: %d",
		res.TLSSubject,
		res.TLSIssuer,
		res.TLSNotAfter.Format(time.RFC3339),
		res.TLSDaysRemaining,
	))
	res.Duration = time.Since(started)
	res.Passed = evaluateTLS(step.Expect, res, &res.Errors) && len(res.Errors) == 0
	return res
}

func (r *Runner) runTCPStep(ctx context.Context, step spec.Step) Result {
	started := time.Now()
	res := Result{
		Step:       step,
		StartedAt:  started,
		Kind:       spec.KindTCP,
		TCPAddress: step.TCP.Address,
	}

	conn, err := r.dialer.DialContext(ctx, "tcp", step.TCP.Address)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		res.Duration = time.Since(started)
		return res
	}
	_ = conn.Close()

	res.Body = []byte("connected " + step.TCP.Address)
	res.Duration = time.Since(started)
	res.Passed = evaluateBodyExpect(step.Expect, res.Body, &res.Errors) && len(res.Errors) == 0
	return res
}

func (r *Runner) lookupDNS(ctx context.Context, query spec.DNSQuery) ([]string, error) {
	resolver := net.DefaultResolver
	if strings.TrimSpace(query.Server) != "" {
		server := query.Server
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return r.dialer.DialContext(ctx, "udp", server)
			},
		}
	}

	var answers []string
	switch query.Type {
	case "A":
		ips, err := resolver.LookupIP(ctx, "ip4", query.Name)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			answers = append(answers, ip.String())
		}
	case "AAAA":
		ips, err := resolver.LookupIP(ctx, "ip6", query.Name)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			answers = append(answers, ip.String())
		}
	case "CNAME":
		cname, err := resolver.LookupCNAME(ctx, query.Name)
		if err != nil {
			return nil, err
		}
		answers = append(answers, strings.TrimSuffix(cname, "."))
	case "MX":
		mx, err := resolver.LookupMX(ctx, query.Name)
		if err != nil {
			return nil, err
		}
		for _, record := range mx {
			answers = append(answers, fmt.Sprintf("%d %s", record.Pref, strings.TrimSuffix(record.Host, ".")))
		}
	case "NS":
		ns, err := resolver.LookupNS(ctx, query.Name)
		if err != nil {
			return nil, err
		}
		for _, record := range ns {
			answers = append(answers, strings.TrimSuffix(record.Host, "."))
		}
	case "TXT":
		txt, err := resolver.LookupTXT(ctx, query.Name)
		if err != nil {
			return nil, err
		}
		answers = append(answers, txt...)
	default:
		return nil, fmt.Errorf("unsupported dns record type %q", query.Type)
	}

	sort.Strings(answers)
	return answers, nil
}

func evaluateHTTP(expect spec.Expect, status int, headers http.Header, body []byte, errs *[]string) bool {
	ok := true
	if expect.Status != 0 && status != expect.Status {
		ok = false
		*errs = append(*errs, fmt.Sprintf("expected status %d, got %d", expect.Status, status))
	}

	if !evaluateBodyExpect(expect, body, errs) {
		ok = false
	}
	if !evaluateHeaderExpect(expect, headers, errs) {
		ok = false
	}

	return ok
}

func evaluateDNS(expect spec.Expect, answers []string, body []byte, errs *[]string) bool {
	ok := true
	if !evaluateBodyExpect(expect, body, errs) {
		ok = false
	}

	answerText := strings.Join(answers, "\n")
	for _, s := range expect.AnswerContains {
		if s == "" {
			continue
		}
		if !strings.Contains(answerText, s) {
			ok = false
			*errs = append(*errs, fmt.Sprintf("expected answers to contain %q", s))
		}
	}
	for _, s := range expect.AnswerAbsent {
		if s == "" {
			continue
		}
		if strings.Contains(answerText, s) {
			ok = false
			*errs = append(*errs, fmt.Sprintf("expected answers to NOT contain %q", s))
		}
	}

	return ok
}

func evaluateTLS(expect spec.Expect, res Result, errs *[]string) bool {
	ok := evaluateBodyExpect(expect, res.Body, errs)
	if expect.DaysRemainingAtLeast != nil && res.TLSDaysRemaining < *expect.DaysRemainingAtLeast {
		ok = false
		*errs = append(*errs, fmt.Sprintf(
			"expected days remaining >= %d, got %d",
			*expect.DaysRemainingAtLeast,
			res.TLSDaysRemaining,
		))
	}
	return ok
}

func evaluateBodyExpect(expect spec.Expect, body []byte, errs *[]string) bool {
	ok := true
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

	return ok
}

func evaluateHeaderExpect(expect spec.Expect, headers http.Header, errs *[]string) bool {
	if len(expect.HeaderHas) == 0 {
		return true
	}

	ok := true
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

	return ok
}
