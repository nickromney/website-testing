package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nickromney/website-testing/internal/runner"
	"github.com/nickromney/website-testing/internal/spec"
)

type status int

const (
	statusPending status = iota
	statusRunning
	statusOK
	statusFail
)

type stepFinishedMsg struct {
	idx   int
	runID int
	res   runner.Result
}

type model struct {
	spec     *spec.Spec
	specPath string
	runner   *runner.Runner

	width  int
	height int

	selected int
	results  []runner.Result
	states   []status

	runID   int
	queue   []int
	running bool
	help    bool

	statusText string
}

func Run(specDoc *spec.Spec, r *runner.Runner, specPath string) error {
	m := newModel(specDoc, r, specPath)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(specDoc *spec.Spec, r *runner.Runner, specPath string) model {
	m := model{
		spec:       specDoc,
		specPath:   specPath,
		runner:     r,
		results:    make([]runner.Result, len(specDoc.Steps)),
		states:     make([]status, len(specDoc.Steps)),
		statusText: "Press ? for help",
	}

	if len(specDoc.Steps) > 0 {
		m.runID = 1
		m.running = true
		m.states[0] = statusRunning
		m.queue = makeRange(1, len(specDoc.Steps))
		m.statusText = "Running all steps"
	}

	return m
}

func (m model) Init() tea.Cmd {
	if len(m.spec.Steps) == 0 {
		return tea.WindowSize()
	}
	return tea.Batch(
		tea.WindowSize(),
		runStepCmd(m.runner, m.spec.Steps[0], 0, m.runID),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.updateKey(msg)

	case stepFinishedMsg:
		if msg.runID != m.runID {
			return m, nil
		}

		m.results[msg.idx] = msg.res
		if msg.res.Passed {
			m.states[msg.idx] = statusOK
		} else {
			m.states[msg.idx] = statusFail
		}

		if len(m.queue) == 0 {
			m.running = false
			if m.failedCount() == 0 {
				m.statusText = "All steps passed"
			} else {
				m.statusText = fmt.Sprintf("%d step(s) failed", m.failedCount())
			}
			return m, nil
		}

		next := m.queue[0]
		m.queue = m.queue[1:]
		m.states[next] = statusRunning
		return m, runStepCmd(m.runner, m.spec.Steps[next], next, m.runID)
	}

	return m, nil
}

func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "?":
		m.help = !m.help
		return m, nil
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case "down", "j":
		if m.selected < len(m.spec.Steps)-1 {
			m.selected++
		}
		return m, nil
	case "g":
		m.selected = 0
		return m, nil
	case "G":
		if len(m.spec.Steps) > 0 {
			m.selected = len(m.spec.Steps) - 1
		}
		return m, nil
	case "r", "enter":
		return m.rerunSelected()
	case "R":
		return m.rerunAll()
	}

	return m, nil
}

func (m model) rerunSelected() (tea.Model, tea.Cmd) {
	if len(m.spec.Steps) == 0 {
		return m, nil
	}

	m.runID++
	m.queue = nil
	m.running = true
	m.results[m.selected] = runner.Result{}
	m.states[m.selected] = statusRunning
	m.statusText = fmt.Sprintf("Re-running %s", m.spec.Steps[m.selected].Name)

	return m, runStepCmd(m.runner, m.spec.Steps[m.selected], m.selected, m.runID)
}

func (m model) rerunAll() (tea.Model, tea.Cmd) {
	if len(m.spec.Steps) == 0 {
		return m, nil
	}

	m.runID++
	m.selected = 0
	m.results = make([]runner.Result, len(m.spec.Steps))
	m.states = make([]status, len(m.spec.Steps))
	m.queue = makeRange(1, len(m.spec.Steps))
	m.running = true
	m.states[0] = statusRunning
	m.statusText = "Running all steps"

	return m, runStepCmd(m.runner, m.spec.Steps[0], 0, m.runID)
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading smoke-go..."
	}

	header := titleStyle.Width(m.width).Render(m.headerText())
	statusBar := statusBarStyle.Width(m.width).Render(m.statusBarText())
	bodyHeight := maxInt(1, m.height-2)

	var body string
	if m.width < 90 {
		body = lipgloss.JoinVertical(lipgloss.Left,
			renderPane("Steps", m.stepListText(), m.width, bodyHeight/2, true),
			renderPane("Details", m.detailText(), m.width, maxInt(1, bodyHeight-bodyHeight/2), false),
		)
	} else {
		listWidth := maxInt(28, minInt(42, m.width/3))
		detailWidth := maxInt(20, m.width-listWidth)
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			renderPane("Steps", m.stepListText(), listWidth, bodyHeight, true),
			renderPane("Details", m.detailText(), detailWidth, bodyHeight, false),
		)
	}

	screen := lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
	if m.help {
		return overlay(screen, m.helpPanel())
	}
	return screen
}

func (m model) headerText() string {
	name := strings.TrimSpace(m.spec.Name)
	if name == "" {
		name = filepath.Base(m.specPath)
	}

	state := "idle"
	if m.running {
		state = "running"
	} else if m.failedCount() == 0 {
		state = "passed"
	} else {
		state = "failed"
	}

	return fmt.Sprintf("smoke-go  %s  [%s]  pass:%d fail:%d pending:%d",
		name,
		state,
		m.passedCount(),
		m.failedCount(),
		m.pendingCount(),
	)
}

func (m model) statusBarText() string {
	hints := "j/k move  enter rerun step  R rerun all  ? help  q quit"
	if strings.TrimSpace(m.statusText) == "" {
		return hints
	}
	return fmt.Sprintf("%s  |  %s", hints, m.statusText)
}

func (m model) stepListText() string {
	var b strings.Builder
	for i, step := range m.spec.Steps {
		cursor := " "
		if i == m.selected {
			cursor = ">"
		}

		statusText := statusGlyph(m.states[i])
		detail := ""
		if !m.results[i].StartedAt.IsZero() {
			detail = m.results[i].Duration.Round(time.Millisecond).String()
		}

		line := fmt.Sprintf("%s %s %-30s %s", cursor, statusText, truncateLabel(stepDisplayName(step), 30), detail)
		b.WriteString(strings.TrimRight(line, " "))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m model) detailText() string {
	if len(m.spec.Steps) == 0 {
		return "No steps in spec."
	}

	step := m.spec.Steps[m.selected]
	res := m.results[m.selected]

	sections := []string{
		fmt.Sprintf("Step %d/%d", m.selected+1, len(m.spec.Steps)),
		fmt.Sprintf("Kind: %s", step.Kind),
		fmt.Sprintf("Status: %s", stateText(m.states[m.selected], res.Passed)),
	}

	switch step.Kind {
	case spec.KindDNS:
		sections = append(sections,
			"Query:",
			"  name: "+step.DNS.Name,
			"  type: "+step.DNS.Type,
		)
		if strings.TrimSpace(step.DNS.Server) != "" {
			sections = append(sections, "  server: "+step.DNS.Server)
		}
	case spec.KindTLS:
		sections = append(sections,
			"Target:",
			"  address: "+step.TLS.Address,
			"  server_name: "+step.TLS.ServerName,
		)
	case spec.KindTCP:
		sections = append(sections,
			"Target:",
			"  address: "+step.TCP.Address,
		)
	default:
		sections = append(sections, fmt.Sprintf("Request: %s %s", step.Request.Method, step.Request.URL))

		if len(step.Request.Headers) > 0 {
			sections = append(sections, "Request headers:")
			keys := make([]string, 0, len(step.Request.Headers))
			for k := range step.Request.Headers {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				sections = append(sections, "  "+k+": "+step.Request.Headers[k])
			}
		}

		if strings.TrimSpace(step.Request.Body) != "" {
			sections = append(sections, "Request body:")
			sections = append(sections, indentBlock(step.Request.Body)...)
		}
	}

	expectLines := formatExpect(step.Expect)
	if len(expectLines) > 0 {
		sections = append(sections, "Expect:")
		sections = append(sections, expectLines...)
	}

	if res.StartedAt.IsZero() {
		sections = append(sections, "Result: not run yet")
		return strings.Join(sections, "\n")
	}

	sections = append(sections, "Result:")
	switch res.Kind {
	case spec.KindDNS:
		sections = append(sections,
			fmt.Sprintf("  duration: %s", res.Duration.Round(time.Millisecond)),
			fmt.Sprintf("  answers: %d", len(res.DNSAnswers)),
		)
		if strings.TrimSpace(res.DNSResolver) != "" {
			sections = append(sections, "  resolver: "+res.DNSResolver)
		}
		if len(res.DNSAnswers) > 0 {
			sections = append(sections, "Answers:")
			for _, answer := range res.DNSAnswers {
				sections = append(sections, "  - "+answer)
			}
		}
	case spec.KindTLS:
		sections = append(sections,
			fmt.Sprintf("  duration: %s", res.Duration.Round(time.Millisecond)),
			fmt.Sprintf("  address: %s", res.TLSAddress),
			fmt.Sprintf("  server_name: %s", res.TLSServerName),
			fmt.Sprintf("  days remaining: %d", res.TLSDaysRemaining),
		)
		if !res.TLSNotAfter.IsZero() {
			sections = append(sections, "  not_after: "+res.TLSNotAfter.Format(time.RFC3339))
		}
		if strings.TrimSpace(res.TLSSubject) != "" {
			sections = append(sections, "  subject: "+res.TLSSubject)
		}
		if strings.TrimSpace(res.TLSIssuer) != "" {
			sections = append(sections, "  issuer: "+res.TLSIssuer)
		}
	case spec.KindTCP:
		sections = append(sections,
			fmt.Sprintf("  duration: %s", res.Duration.Round(time.Millisecond)),
			fmt.Sprintf("  address: %s", res.TCPAddress),
		)
	default:
		sections = append(sections,
			fmt.Sprintf("  status code: %d", res.StatusCode),
			fmt.Sprintf("  duration: %s", res.Duration.Round(time.Millisecond)),
		)
	}

	if len(res.Errors) > 0 {
		sections = append(sections, "Errors:")
		for _, e := range res.Errors {
			sections = append(sections, "  - "+e)
		}
	}

	if len(res.Headers) > 0 {
		sections = append(sections, "Response headers:")
		headerKeys := make([]string, 0, len(res.Headers))
		for k := range res.Headers {
			headerKeys = append(headerKeys, k)
		}
		sort.Strings(headerKeys)
		for _, k := range headerKeys {
			sections = append(sections, "  "+k+": "+strings.Join(res.Headers[k], ", "))
		}
	}

	bodyText := strings.TrimSpace(string(res.Body))
	if bodyText != "" {
		sections = append(sections, "Body preview:")
		sections = append(sections, indentBlock(previewBody(bodyText))...)
	}

	return strings.Join(sections, "\n")
}

func stepDisplayName(step spec.Step) string {
	if step.Kind == "" || step.Kind == spec.KindHTTP {
		return step.Name
	}
	return step.Kind + ": " + step.Name
}

func formatExpect(expect spec.Expect) []string {
	var lines []string
	if expect.Status != 0 {
		lines = append(lines, fmt.Sprintf("  status == %d", expect.Status))
	}
	for _, s := range expect.BodyContains {
		lines = append(lines, fmt.Sprintf("  body contains %q", s))
	}
	for _, s := range expect.BodyAbsent {
		lines = append(lines, fmt.Sprintf("  body excludes %q", s))
	}
	for _, s := range expect.HeaderHas {
		lines = append(lines, fmt.Sprintf("  headers contain %q", s))
	}
	for _, s := range expect.AnswerContains {
		lines = append(lines, fmt.Sprintf("  answers contain %q", s))
	}
	for _, s := range expect.AnswerAbsent {
		lines = append(lines, fmt.Sprintf("  answers exclude %q", s))
	}
	if expect.DaysRemainingAtLeast != nil {
		lines = append(lines, fmt.Sprintf("  days remaining >= %d", *expect.DaysRemainingAtLeast))
	}
	return lines
}

func (m model) helpPanel() string {
	content := strings.Join([]string{
		"smoke-go",
		"",
		"j / k or arrows : move selection",
		"enter / r       : rerun selected step",
		"R               : rerun full spec",
		"?               : toggle help",
		"q / esc         : quit",
		"",
		"The left pane tracks step status and timings.",
		"The right pane shows request, expectations, and last response details.",
	}, "\n")

	panelWidth := minInt(maxInt(56, m.width/2), maxInt(56, m.width-4))
	if panelWidth > m.width {
		panelWidth = m.width
	}

	return helpPanelStyle.
		Width(panelWidth).
		Render(fitContent(content, panelWidth-4, 10))
}

func renderPane(title string, content string, width int, height int, active bool) string {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	style := paneStyle.Copy().Width(width).Height(height)
	if active {
		style = style.BorderForeground(activeBorderColor)
	} else {
		style = style.BorderForeground(inactiveBorderColor)
	}

	body := fitContent(content, width-4, maxInt(1, height-3))
	full := titleLine(title, width-4) + "\n" + body
	return style.Render(full)
}

func overlay(screen string, panel string) string {
	lines := strings.Split(screen, "\n")
	if len(lines) == 0 {
		return panel
	}
	panelLines := strings.Split(panel, "\n")
	panelHeight := len(panelLines)
	panelWidth := 0
	for _, line := range panelLines {
		panelWidth = maxInt(panelWidth, lipgloss.Width(line))
	}

	row := maxInt(0, (len(lines)-panelHeight)/2)
	col := maxInt(0, (lipgloss.Width(lines[0])-panelWidth)/2)

	for i, line := range panelLines {
		r := row + i
		if r >= len(lines) {
			break
		}

		left := strings.Repeat(" ", col)
		rightWidth := maxInt(0, lipgloss.Width(lines[r])-col-lipgloss.Width(line))
		lines[r] = padWidth(left+line+strings.Repeat(" ", rightWidth), lipgloss.Width(lines[r]))
	}

	return strings.Join(lines, "\n")
}

func runStepCmd(r *runner.Runner, step spec.Step, idx int, runID int) tea.Cmd {
	return func() tea.Msg {
		res := r.RunStep(context.Background(), step)
		return stepFinishedMsg{idx: idx, runID: runID, res: res}
	}
}

func makeRange(start int, end int) []int {
	if end <= start {
		return nil
	}

	out := make([]int, 0, end-start)
	for i := start; i < end; i++ {
		out = append(out, i)
	}
	return out
}

func (m model) passedCount() int {
	count := 0
	for _, s := range m.states {
		if s == statusOK {
			count++
		}
	}
	return count
}

func (m model) failedCount() int {
	count := 0
	for _, s := range m.states {
		if s == statusFail {
			count++
		}
	}
	return count
}

func (m model) pendingCount() int {
	count := 0
	for _, s := range m.states {
		if s == statusPending || s == statusRunning {
			count++
		}
	}
	return count
}

func statusGlyph(s status) string {
	switch s {
	case statusRunning:
		return "[~]"
	case statusOK:
		return "[OK]"
	case statusFail:
		return "[!!]"
	default:
		return "[ ]"
	}
}

func stateText(s status, passed bool) string {
	switch s {
	case statusRunning:
		return "running"
	case statusOK:
		return "passed"
	case statusFail:
		if passed {
			return "passed"
		}
		return "failed"
	default:
		return "pending"
	}
}

func truncateLabel(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	if max == 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}

func previewBody(s string) string {
	s = strings.ReplaceAll(s, "\x00", "\uFFFD")
	lines := strings.Split(s, "\n")
	if len(lines) > 12 {
		lines = append(lines[:11], "…")
	}
	for i := range lines {
		lines[i] = truncateLabel(lines[i], 120)
	}
	return strings.Join(lines, "\n")
}

func indentBlock(s string) []string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return lines
}

func fitContent(content string, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	out := make([]string, 0, height)
	for _, line := range lines {
		out = append(out, truncateLabel(line, width))
		if len(out) == height {
			break
		}
	}

	if len(lines) > height && len(out) > 0 {
		out[len(out)-1] = truncateLabel(out[len(out)-1], maxInt(1, width-1)) + "…"
	}

	for len(out) < height {
		out = append(out, "")
	}

	return strings.Join(out, "\n")
}

func padWidth(s string, width int) string {
	current := lipgloss.Width(s)
	if current >= width {
		return s
	}
	return s + strings.Repeat(" ", width-current)
}

func titleLine(title string, width int) string {
	label := strings.ToUpper(strings.TrimSpace(title))
	if label == "" {
		label = "PANE"
	}
	return truncateLabel(label, width)
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
