package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nickromney/website-testing/internal/runner"
	"github.com/nickromney/website-testing/internal/spec"
)

func TestBuilderBuildSpecSupportsAllKinds(t *testing.T) {
	m := newBuilderModel(nil)
	m.specName = "network checks"
	m.baseURL = "https://example.org"
	m.steps = []builderStep{
		{
			Name:               "health",
			Kind:               spec.KindHTTP,
			HTTPMethod:         "post",
			HTTPTarget:         "/health",
			HTTPHeaders:        "Host: example.test | X-Trace: 1",
			HTTPBody:           "{\"ok\":true}",
			HTTPStatus:         "201",
			HTTPBodyContains:   "ok",
			HTTPBodyAbsent:     "panic",
			HTTPHeaderContains: "Content-Type: application/json",
		},
		{
			Name:              "dns",
			Kind:              spec.KindDNS,
			DNSName:           "example.org",
			DNSType:           "txt",
			DNSServer:         "127.0.0.1",
			DNSAnswerContains: "spf",
			DNSAnswerAbsent:   "broken",
		},
		{
			Name:                "tls",
			Kind:                spec.KindTLS,
			TLSAddress:          "example.org",
			TLSDaysRemainingMin: "14",
		},
		{
			Name:       "tcp",
			Kind:       spec.KindTCP,
			TCPAddress: "example.org",
		},
	}

	doc, err := m.buildSpec()
	if err != nil {
		t.Fatalf("buildSpec() err = %v", err)
	}
	if len(doc.Steps) != 4 {
		t.Fatalf("steps = %d", len(doc.Steps))
	}

	httpStep := doc.Steps[0]
	if got := httpStep.Request.Method; got != "POST" {
		t.Fatalf("http method = %q", got)
	}
	if got := httpStep.Request.URL; got != "https://example.org/health" {
		t.Fatalf("http url = %q", got)
	}
	if got := httpStep.Expect.Status; got != 201 {
		t.Fatalf("http status = %d", got)
	}
	if got := httpStep.Request.Headers["Host"]; got != "example.test" {
		t.Fatalf("host header = %q", got)
	}

	dnsStep := doc.Steps[1]
	if got := dnsStep.DNS.Type; got != "TXT" {
		t.Fatalf("dns type = %q", got)
	}
	if got := dnsStep.DNS.Server; got != "127.0.0.1:53" {
		t.Fatalf("dns server = %q", got)
	}

	tlsStep := doc.Steps[2]
	if got := tlsStep.TLS.Address; got != "example.org:443" {
		t.Fatalf("tls address = %q", got)
	}
	if got := tlsStep.TLS.ServerName; got != "example.org" {
		t.Fatalf("tls server name = %q", got)
	}
	if tlsStep.Expect.DaysRemainingAtLeast == nil || *tlsStep.Expect.DaysRemainingAtLeast != 14 {
		t.Fatalf("tls days remaining = %#v", tlsStep.Expect.DaysRemainingAtLeast)
	}

	tcpStep := doc.Steps[3]
	if got := tcpStep.TCP.Address; got != "example.org:80" {
		t.Fatalf("tcp address = %q", got)
	}
}

func TestBuilderUpdateKeyCanRunAndReturn(t *testing.T) {
	r, err := runner.New(250 * time.Millisecond)
	if err != nil {
		t.Fatalf("runner.New() err = %v", err)
	}

	m := newBuilderModel(r)

	nextModel, _ := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = nextModel.(builderModel)
	if len(m.steps) != 2 || m.selectedStep != 1 {
		t.Fatalf("after add: steps=%d selected=%d", len(m.steps), m.selectedStep)
	}

	nextModel, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyTab})
	m = nextModel.(builderModel)
	if m.focus != focusFields {
		t.Fatalf("focus = %v", m.focus)
	}

	m.selectedField = 4
	nextModel, _ = m.updateFieldKey(tea.KeyMsg{Type: tea.KeyRight})
	m = nextModel.(builderModel)
	if got := m.currentStep().Kind; got != spec.KindDNS {
		t.Fatalf("kind = %q", got)
	}
	m.currentStepPtr().DNSName = "example.org"

	runModel, cmd := m.run()
	if cmd == nil {
		t.Fatal("run() returned nil cmd")
	}
	next, ok := runModel.(model)
	if !ok {
		t.Fatalf("run model type = %T", runModel)
	}
	if next.backToBuilder == nil {
		t.Fatal("expected builder back-link")
	}

	backModel, _ := next.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	back, ok := backModel.(builderModel)
	if !ok {
		t.Fatalf("back model type = %T", backModel)
	}
	if !strings.Contains(back.statusText, "Returned") {
		t.Fatalf("statusText = %q", back.statusText)
	}
}

func TestBuilderSaveWritesLoadableSpec(t *testing.T) {
	m := newBuilderModel(nil)
	m.savePath = filepath.Join(t.TempDir(), "nested", "smoke.yaml")

	savedModel, _ := m.save()
	m = savedModel.(builderModel)
	if !strings.Contains(m.statusText, "Saved") {
		t.Fatalf("statusText = %q", m.statusText)
	}

	doc, err := spec.Load(m.savePath)
	if err != nil {
		t.Fatalf("Load(%q) err = %v", m.savePath, err)
	}
	if len(doc.Steps) != 1 {
		t.Fatalf("steps = %d", len(doc.Steps))
	}
	if got := doc.Steps[0].Kind; got != spec.KindHTTP {
		t.Fatalf("kind = %q", got)
	}
}
