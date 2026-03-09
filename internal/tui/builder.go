package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nickromney/website-testing/internal/adhoc"
	"github.com/nickromney/website-testing/internal/runner"
	"github.com/nickromney/website-testing/internal/spec"
)

type builderField int

const (
	fieldTarget builderField = iota
	fieldMethod
	fieldStatus
	fieldBodyContains
	fieldHeaderContains
)

type builderModel struct {
	runner *runner.Runner

	width  int
	height int

	selected builderField
	editing  bool

	target         string
	method         string
	status         string
	bodyContains   string
	headerContains string

	statusText string
}

func RunBuilder(r *runner.Runner) error {
	m := builderModel{
		runner:         r,
		method:         "GET",
		status:         "200",
		statusText:     "Fill in the target, then press r to run",
		selected:       fieldTarget,
		editing:        false,
		target:         "",
		bodyContains:   "",
		headerContains: "",
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m builderModel) Init() tea.Cmd {
	return tea.WindowSize()
}

func (m builderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m builderModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editing {
		switch msg.String() {
		case "esc":
			m.editing = false
			m.statusText = "Edit cancelled"
			return m, nil
		case "enter":
			m.editing = false
			m.statusText = "Field updated"
			return m, nil
		case "backspace":
			m.setSelectedValue(trimLastRune(m.selectedValue()))
			return m, nil
		}

		if text := msg.String(); len(text) == 1 && text >= " " {
			m.setSelectedValue(m.selectedValue() + text)
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.selected > fieldTarget {
			m.selected--
		}
		return m, nil
	case "down", "j":
		if m.selected < fieldHeaderContains {
			m.selected++
		}
		return m, nil
	case "enter":
		m.editing = true
		m.statusText = "Editing field"
		return m, nil
	case "r":
		return m.run()
	case "?":
		m.statusText = "enter edit  r run  q quit"
		return m, nil
	}

	return m, nil
}

func (m builderModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading smoke-go..."
	}

	header := titleStyle.Width(m.width).Render("smoke-go builder  configure a quick HTTP smoke check")
	statusBar := statusBarStyle.Width(m.width).Render("enter edit  r run  q quit  |  " + m.statusText)
	bodyHeight := maxInt(1, m.height-2)
	formWidth := maxInt(32, minInt(44, m.width/2))
	previewWidth := maxInt(20, m.width-formWidth)

	form := renderPane("Fields", m.formText(), formWidth, bodyHeight, true)
	preview := renderPane("Generated Spec", m.previewText(), previewWidth, bodyHeight, false)
	return lipgloss.JoinVertical(lipgloss.Left, header, lipgloss.JoinHorizontal(lipgloss.Top, form, preview), statusBar)
}

func (m builderModel) formText() string {
	fields := []struct {
		id    builderField
		label string
		value string
	}{
		{fieldTarget, "Target", m.target},
		{fieldMethod, "Method", m.method},
		{fieldStatus, "Status", m.status},
		{fieldBodyContains, "Body Contains", m.bodyContains},
		{fieldHeaderContains, "Header Contains", m.headerContains},
	}

	var b strings.Builder
	for _, field := range fields {
		cursor := " "
		if m.selected == field.id {
			cursor = ">"
		}
		value := field.value
		if strings.TrimSpace(value) == "" {
			value = "(empty)"
		}
		if m.editing && m.selected == field.id {
			value += " _"
		}
		b.WriteString(fmt.Sprintf("%s %-16s %s\n", cursor, field.label, value))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m builderModel) previewText() string {
	target, err := adhoc.NormalizeTarget(m.target)
	if err != nil && strings.TrimSpace(m.target) != "" {
		return "Target error:\n  " + err.Error()
	}

	method := strings.TrimSpace(m.method)
	if method == "" {
		method = "GET"
	}
	status := strings.TrimSpace(m.status)
	if status == "" {
		status = "200"
	}

	lines := []string{
		"name: quick smoke",
		"steps:",
		"  - name: quick smoke",
		"    kind: http",
		"    request:",
		"      method: " + strings.ToUpper(method),
		"      url: " + targetOrPlaceholder(target),
		"    expect:",
		"      status: " + status,
	}
	if strings.TrimSpace(m.bodyContains) != "" {
		lines = append(lines, "      body_contains:")
		lines = append(lines, "        - "+m.bodyContains)
	}
	if strings.TrimSpace(m.headerContains) != "" {
		lines = append(lines, "      header_contains:")
		lines = append(lines, "        - "+m.headerContains)
	}
	return strings.Join(lines, "\n")
}

func (m builderModel) run() (tea.Model, tea.Cmd) {
	statusCode, err := parseStatus(m.status)
	if err != nil {
		m.statusText = err.Error()
		return m, nil
	}

	var bodyContains []string
	if strings.TrimSpace(m.bodyContains) != "" {
		bodyContains = []string{strings.TrimSpace(m.bodyContains)}
	}
	var headerContains []string
	if strings.TrimSpace(m.headerContains) != "" {
		headerContains = []string{strings.TrimSpace(m.headerContains)}
	}

	step, err := adhoc.HTTPCheck(m.target, adhoc.HTTPOptions{
		Method:         m.method,
		Status:         statusCode,
		BodyContains:   bodyContains,
		HeaderContains: headerContains,
	})
	if err != nil {
		m.statusText = err.Error()
		return m, nil
	}

	specDoc := &spec.Spec{
		Name:  "quick smoke",
		Steps: []spec.Step{step},
	}
	next := newModel(specDoc, m.runner, "interactive")
	if m.width != 0 {
		next.width = m.width
	}
	if m.height != 0 {
		next.height = m.height
	}
	return next, next.Init()
}

func (m builderModel) selectedValue() string {
	switch m.selected {
	case fieldMethod:
		return m.method
	case fieldStatus:
		return m.status
	case fieldBodyContains:
		return m.bodyContains
	case fieldHeaderContains:
		return m.headerContains
	default:
		return m.target
	}
}

func (m *builderModel) setSelectedValue(value string) {
	switch m.selected {
	case fieldMethod:
		m.method = value
	case fieldStatus:
		m.status = value
	case fieldBodyContains:
		m.bodyContains = value
	case fieldHeaderContains:
		m.headerContains = value
	default:
		m.target = value
	}
}

func trimLastRune(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	return string(runes[:len(runes)-1])
}

func parseStatus(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 200, nil
	}
	var status int
	if _, err := fmt.Sscanf(s, "%d", &status); err != nil || status <= 0 {
		return 0, fmt.Errorf("status must be a positive integer")
	}
	return status, nil
}

func targetOrPlaceholder(target string) string {
	if strings.TrimSpace(target) == "" {
		return "<target>"
	}
	return target
}
