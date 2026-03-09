package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nickromney/website-testing/internal/adhoc"
	"github.com/nickromney/website-testing/internal/runner"
	"github.com/nickromney/website-testing/internal/spec"
)

type builderFocus int

const (
	focusSteps builderFocus = iota
	focusFields
)

type builderStep struct {
	Name string
	Kind string

	HTTPMethod         string
	HTTPTarget         string
	HTTPHeaders        string
	HTTPBody           string
	HTTPStatus         string
	HTTPBodyContains   string
	HTTPBodyAbsent     string
	HTTPHeaderContains string

	DNSName           string
	DNSType           string
	DNSServer         string
	DNSAnswerContains string
	DNSAnswerAbsent   string

	TLSAddress          string
	TLSServerName       string
	TLSDaysRemainingMin string

	TCPAddress string
}

type builderFieldDef struct {
	Label       string
	Placeholder string
	Options     []string
	Hint        string
	Get         func(*builderModel) string
	Set         func(*builderModel, string)
}

type builderModel struct {
	runner *runner.Runner

	width  int
	height int

	focus         builderFocus
	selectedStep  int
	selectedField int
	editing       bool
	help          bool

	specName string
	baseURL  string
	savePath string
	steps    []builderStep

	statusText string
}

func RunBuilder(r *runner.Runner) error {
	p := tea.NewProgram(newBuilderModel(r), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newBuilderModel(r *runner.Runner) builderModel {
	return builderModel{
		runner:       r,
		focus:        focusSteps,
		specName:     "quick smoke",
		steps:        []builderStep{newBuilderStep(spec.KindHTTP, 1)},
		statusText:   "Tab switches panes. a adds a step. r runs. s saves.",
		selectedStep: 0,
	}
}

func newBuilderStep(kind string, index int) builderStep {
	kind = normaliseDraftKind(kind)

	step := builderStep{
		Name:                fmt.Sprintf("%s step %d", kind, index),
		Kind:                kind,
		HTTPMethod:          "GET",
		HTTPStatus:          "200",
		DNSType:             "A",
		TLSDaysRemainingMin: "7",
	}

	switch kind {
	case spec.KindDNS:
		step.DNSName = "example.org"
	case spec.KindTLS:
		step.TLSAddress = "example.org:443"
	case spec.KindTCP:
		step.TCPAddress = "example.org:443"
	default:
		step.HTTPTarget = "https://example.org/"
	}

	return step
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
	if m.help {
		switch msg.String() {
		case "?", "esc":
			m.help = false
			m.statusText = "Help closed"
		}
		return m, nil
	}

	if m.editing {
		return m.updateEditing(msg)
	}

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "?":
		m.help = true
		return m, nil
	case "tab":
		m.focus = nextBuilderFocus(m.focus)
		m.clampSelection()
		m.statusText = "Switched pane"
		return m, nil
	case "shift+tab":
		m.focus = previousBuilderFocus(m.focus)
		m.clampSelection()
		m.statusText = "Switched pane"
		return m, nil
	case "r":
		return m.run()
	case "s":
		return m.save()
	}

	switch m.focus {
	case focusFields:
		return m.updateFieldKey(msg)
	default:
		return m.updateStepKey(msg)
	}
}

func (m builderModel) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.setSelectedFieldValue(trimLastRune(m.selectedFieldValue()))
		return m, nil
	case "ctrl+u":
		m.setSelectedFieldValue("")
		return m, nil
	}

	if text := msg.String(); len(text) == 1 && text >= " " {
		m.setSelectedFieldValue(m.selectedFieldValue() + text)
		return m, nil
	}

	return m, nil
}

func (m builderModel) updateStepKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedStep > 0 {
			m.selectedStep--
			m.statusText = "Selected previous step"
		}
	case "down", "j":
		if m.selectedStep < len(m.steps)-1 {
			m.selectedStep++
			m.statusText = "Selected next step"
		}
	case "g":
		m.selectedStep = 0
		m.statusText = "Jumped to first step"
	case "G":
		if len(m.steps) > 0 {
			m.selectedStep = len(m.steps) - 1
			m.statusText = "Jumped to last step"
		}
	case "a":
		m.addStep()
	case "x", "d":
		m.deleteStep()
	case "J":
		m.moveStep(1)
	case "K":
		m.moveStep(-1)
	case "enter":
		m.focus = focusFields
		m.clampSelection()
		m.statusText = "Editing fields"
	}

	return m, nil
}

func (m builderModel) updateFieldKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fields := m.fieldDefs()
	if len(fields) == 0 {
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.selectedField > 0 {
			m.selectedField--
			m.statusText = m.currentFieldHint()
		}
	case "down", "j":
		if m.selectedField < len(fields)-1 {
			m.selectedField++
			m.statusText = m.currentFieldHint()
		}
	case "g":
		m.selectedField = 0
		m.statusText = m.currentFieldHint()
	case "G":
		m.selectedField = len(fields) - 1
		m.statusText = m.currentFieldHint()
	case "left", "h":
		if m.cycleSelectedField(-1) {
			m.statusText = "Field updated"
		}
	case "right", "l":
		if m.cycleSelectedField(1) {
			m.statusText = "Field updated"
		}
	case "ctrl+u":
		m.setSelectedFieldValue("")
		m.statusText = "Field cleared"
	case "enter":
		if m.cycleSelectedField(1) {
			m.statusText = "Field updated"
		} else {
			m.editing = true
			m.statusText = "Editing field"
		}
	}

	return m, nil
}

func (m builderModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading smoke builder..."
	}

	header := titleStyle.Width(m.width).Render(m.headerText())
	statusBar := statusBarStyle.Width(m.width).Render(m.statusBarText())
	bodyHeight := maxInt(1, m.height-2)

	var body string
	if m.width < 110 {
		topHeight := maxInt(8, bodyHeight/2)
		bottomHeight := maxInt(1, bodyHeight-topHeight)
		top := lipgloss.JoinHorizontal(lipgloss.Top,
			renderPane("Steps", m.stepListText(), maxInt(24, m.width/2), topHeight, m.focus == focusSteps),
			renderPane("Fields", m.fieldText(), maxInt(24, m.width-maxInt(24, m.width/2)), topHeight, m.focus == focusFields),
		)
		bottom := renderPane("YAML Preview", m.previewText(), m.width, bottomHeight, false)
		body = lipgloss.JoinVertical(lipgloss.Left, top, bottom)
	} else {
		stepWidth := maxInt(26, minInt(34, m.width/4))
		fieldWidth := maxInt(36, minInt(48, m.width/3))
		previewWidth := maxInt(28, m.width-stepWidth-fieldWidth)
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			renderPane("Steps", m.stepListText(), stepWidth, bodyHeight, m.focus == focusSteps),
			renderPane("Fields", m.fieldText(), fieldWidth, bodyHeight, m.focus == focusFields),
			renderPane("YAML Preview", m.previewText(), previewWidth, bodyHeight, false),
		)
	}

	screen := lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
	if m.help {
		return overlay(screen, m.helpPanel())
	}
	return screen
}

func (m builderModel) headerText() string {
	name := strings.TrimSpace(m.specName)
	if name == "" {
		name = "untitled spec"
	}

	save := strings.TrimSpace(m.savePath)
	if save == "" {
		save = "(unsaved)"
	}

	return fmt.Sprintf(
		"smoke builder  %s  steps:%d  focus:%s  save:%s",
		name,
		len(m.steps),
		m.focus.String(),
		save,
	)
}

func (m builderModel) statusBarText() string {
	if m.editing {
		return "type text  enter save field  esc cancel  ctrl+u clear"
	}

	hints := "tab switch pane  a add  x delete  J/K move  enter edit  r run  s save  ? help  q quit"
	if strings.TrimSpace(m.statusText) == "" {
		return hints
	}
	return hints + "  |  " + m.statusText
}

func (m builderModel) stepListText() string {
	if len(m.steps) == 0 {
		return "No steps yet.\n\nPress a to add the first step."
	}

	var b strings.Builder
	for i, step := range m.steps {
		cursor := " "
		if i == m.selectedStep {
			if m.focus == focusSteps {
				cursor = ">"
			} else {
				cursor = "•"
			}
		}

		kind := strings.ToUpper(normaliseDraftKind(step.Kind))
		name := strings.TrimSpace(step.Name)
		if name == "" {
			name = fmt.Sprintf("step-%d", i+1)
		}
		line := fmt.Sprintf("%s %-4s %-18s %s", cursor, kind, truncateLabel(name, 18), truncateLabel(stepSummary(step), 24))
		b.WriteString(strings.TrimRight(line, " "))
		b.WriteByte('\n')
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m builderModel) fieldText() string {
	fields := m.fieldDefs()
	if len(fields) == 0 {
		return "No fields available."
	}

	var b strings.Builder
	b.WriteString("Spec\n")
	for i := 0; i < minInt(3, len(fields)); i++ {
		b.WriteString(m.renderFieldLine(i, fields[i]))
		b.WriteByte('\n')
	}

	if len(fields) > 3 {
		stepName := strings.TrimSpace(m.currentStep().Name)
		if stepName == "" {
			stepName = fmt.Sprintf("step-%d", m.selectedStep+1)
		}
		b.WriteByte('\n')
		b.WriteString("Selected Step\n")
		b.WriteString("  ")
		b.WriteString(stepName)
		b.WriteByte('\n')
		for i := 3; i < len(fields); i++ {
			b.WriteString(m.renderFieldLine(i, fields[i]))
			b.WriteByte('\n')
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m builderModel) renderFieldLine(index int, field builderFieldDef) string {
	cursor := " "
	if index == m.selectedField {
		if m.focus == focusFields {
			cursor = ">"
		} else {
			cursor = "•"
		}
	}

	value := field.Get(&m)
	if strings.TrimSpace(value) == "" {
		if strings.TrimSpace(field.Placeholder) == "" {
			value = "(empty)"
		} else {
			value = "(" + field.Placeholder + ")"
		}
	}
	if m.editing && index == m.selectedField {
		value += " _"
	}

	return fmt.Sprintf("%s %-18s %s", cursor, field.Label, value)
}

func (m builderModel) previewText() string {
	doc, err := m.buildSpec()
	if err == nil {
		data, marshalErr := spec.Marshal(doc)
		if marshalErr == nil {
			return strings.TrimRight(string(data), "\n")
		}
		err = marshalErr
	}

	return draftPreviewText(m, err)
}

func (m builderModel) helpPanel() string {
	content := strings.Join([]string{
		"smoke builder",
		"",
		"Tab / Shift+Tab : switch between the step list and field editor",
		"j / k           : move within the active pane",
		"enter           : edit a text field or cycle an option field",
		"left / right    : cycle kind, method, and DNS type fields",
		"a               : add a new step after the current one",
		"x               : delete the current step",
		"J / K           : move the current step down or up",
		"s               : save the current spec to disk",
		"r               : run the current spec in the results TUI",
		"",
		"Use | to separate repeated values in list fields.",
		"HTTP targets accept full URLs or /paths when base_url is set.",
		"After running, press b in the results view to return here.",
	}, "\n")

	panelWidth := minInt(maxInt(64, m.width/2), maxInt(64, m.width-4))
	if panelWidth > m.width {
		panelWidth = m.width
	}

	return helpPanelStyle.
		Width(panelWidth).
		Render(fitContent(content, panelWidth-4, 14))
}

func (m builderModel) buildSpec() (*spec.Spec, error) {
	if len(m.steps) == 0 {
		return nil, fmt.Errorf("spec has no steps")
	}

	doc := &spec.Spec{
		Name:    strings.TrimSpace(m.specName),
		BaseURL: strings.TrimSpace(m.baseURL),
		Steps:   make([]spec.Step, 0, len(m.steps)),
	}

	for i, draft := range m.steps {
		step, err := m.buildStep(i, draft)
		if err != nil {
			return nil, err
		}
		doc.Steps = append(doc.Steps, step)
	}

	if err := spec.Normalise(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (m builderModel) buildStep(index int, draft builderStep) (spec.Step, error) {
	kind := normaliseDraftKind(draft.Kind)
	name := strings.TrimSpace(draft.Name)
	if name == "" {
		name = fmt.Sprintf("step-%d", index+1)
	}

	step := spec.Step{
		Name: name,
		Kind: kind,
	}

	switch kind {
	case spec.KindDNS:
		step.DNS = spec.DNSQuery{
			Name:   strings.TrimSpace(draft.DNSName),
			Type:   strings.TrimSpace(draft.DNSType),
			Server: strings.TrimSpace(draft.DNSServer),
		}
		step.Expect.AnswerContains = splitBuilderList(draft.DNSAnswerContains)
		step.Expect.AnswerAbsent = splitBuilderList(draft.DNSAnswerAbsent)
	case spec.KindTLS:
		step.TLS = spec.TLSProbe{
			Address:    strings.TrimSpace(draft.TLSAddress),
			ServerName: strings.TrimSpace(draft.TLSServerName),
		}
		days, err := parseBuilderOptionalPositiveInt(draft.TLSDaysRemainingMin, "days remaining")
		if err != nil {
			return spec.Step{}, fmt.Errorf("%s: %w", name, err)
		}
		step.Expect.DaysRemainingAtLeast = days
	case spec.KindTCP:
		step.TCP = spec.TCPProbe{
			Address: strings.TrimSpace(draft.TCPAddress),
		}
	default:
		status, err := parseBuilderPositiveInt(draft.HTTPStatus, "status", 200)
		if err != nil {
			return spec.Step{}, fmt.Errorf("%s: %w", name, err)
		}
		headers, err := parseBuilderHeaders(draft.HTTPHeaders)
		if err != nil {
			return spec.Step{}, fmt.Errorf("%s: %w", name, err)
		}
		target, err := normaliseBuilderTarget(draft.HTTPTarget)
		if err != nil {
			return spec.Step{}, fmt.Errorf("%s: %w", name, err)
		}

		step.Request = spec.Request{
			Method:  strings.TrimSpace(draft.HTTPMethod),
			Headers: headers,
			Body:    draft.HTTPBody,
		}
		if strings.HasPrefix(target, "/") {
			step.Request.Path = target
		} else {
			step.Request.URL = target
		}
		step.Expect.Status = status
		step.Expect.BodyContains = splitBuilderList(draft.HTTPBodyContains)
		step.Expect.BodyAbsent = splitBuilderList(draft.HTTPBodyAbsent)
		step.Expect.HeaderHas = splitBuilderList(draft.HTTPHeaderContains)
	}

	return step, nil
}

func (m builderModel) run() (tea.Model, tea.Cmd) {
	doc, err := m.buildSpec()
	if err != nil {
		m.statusText = err.Error()
		return m, nil
	}

	specPath := strings.TrimSpace(m.savePath)
	if specPath == "" {
		specPath = "interactive builder"
	}

	next := newModel(doc, m.runner, specPath)
	back := m
	next.backToBuilder = &back
	if m.width != 0 {
		next.width = m.width
	}
	if m.height != 0 {
		next.height = m.height
	}
	return next, next.Init()
}

func (m builderModel) save() (tea.Model, tea.Cmd) {
	doc, err := m.buildSpec()
	if err != nil {
		m.statusText = err.Error()
		return m, nil
	}

	path := strings.TrimSpace(m.savePath)
	if path == "" {
		path = "smoke.yaml"
		m.savePath = path
	}

	data, err := spec.Marshal(doc)
	if err != nil {
		m.statusText = err.Error()
		return m, nil
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			m.statusText = "save failed: " + err.Error()
			return m, nil
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		m.statusText = "save failed: " + err.Error()
		return m, nil
	}

	m.statusText = "Saved " + path
	return m, nil
}

func (m *builderModel) addStep() {
	insertAt := m.selectedStep + 1
	if insertAt < 0 || insertAt > len(m.steps) {
		insertAt = len(m.steps)
	}

	kind := spec.KindHTTP
	if len(m.steps) > 0 {
		kind = normaliseDraftKind(m.steps[m.selectedStep].Kind)
	}
	step := newBuilderStep(kind, len(m.steps)+1)
	m.steps = append(m.steps[:insertAt], append([]builderStep{step}, m.steps[insertAt:]...)...)
	m.selectedStep = insertAt
	m.focus = focusSteps
	m.statusText = "Added step"
}

func (m *builderModel) deleteStep() {
	if len(m.steps) <= 1 {
		m.steps[0] = newBuilderStep(spec.KindHTTP, 1)
		m.selectedStep = 0
		m.selectedField = 0
		m.focus = focusSteps
		m.statusText = "Reset to a single HTTP step"
		return
	}

	idx := m.selectedStep
	m.steps = append(m.steps[:idx], m.steps[idx+1:]...)
	if m.selectedStep >= len(m.steps) {
		m.selectedStep = len(m.steps) - 1
	}
	m.clampSelection()
	m.statusText = "Deleted step"
}

func (m *builderModel) moveStep(delta int) {
	next := m.selectedStep + delta
	if next < 0 || next >= len(m.steps) {
		return
	}
	m.steps[m.selectedStep], m.steps[next] = m.steps[next], m.steps[m.selectedStep]
	m.selectedStep = next
	m.statusText = "Moved step"
}

func (m builderModel) fieldDefs() []builderFieldDef {
	step := m.currentStepPtr()
	if step == nil {
		return nil
	}

	fields := []builderFieldDef{
		{
			Label:       "Spec Name",
			Placeholder: "quick smoke",
			Hint:        "Top-level spec name shown in the runner header.",
			Get:         func(m *builderModel) string { return m.specName },
			Set:         func(m *builderModel, value string) { m.specName = value },
		},
		{
			Label:       "Base URL",
			Placeholder: "https://example.org",
			Hint:        "Optional base_url for HTTP /path targets.",
			Get:         func(m *builderModel) string { return m.baseURL },
			Set:         func(m *builderModel, value string) { m.baseURL = value },
		},
		{
			Label:       "Save Path",
			Placeholder: "smoke.yaml",
			Hint:        "Used when you press s to save the current spec.",
			Get:         func(m *builderModel) string { return m.savePath },
			Set:         func(m *builderModel, value string) { m.savePath = value },
		},
		{
			Label:       "Step Name",
			Placeholder: fmt.Sprintf("step-%d", m.selectedStep+1),
			Hint:        "Human-friendly label shown in the runner.",
			Get:         func(m *builderModel) string { return m.currentStep().Name },
			Set:         func(m *builderModel, value string) { m.currentStepPtr().Name = value },
		},
		{
			Label:       "Kind",
			Placeholder: spec.KindHTTP,
			Options:     []string{spec.KindHTTP, spec.KindDNS, spec.KindTLS, spec.KindTCP},
			Hint:        "Cycles the selected step between HTTP, DNS, TLS, and TCP.",
			Get:         func(m *builderModel) string { return normaliseDraftKind(m.currentStep().Kind) },
			Set: func(m *builderModel, value string) {
				m.currentStepPtr().Kind = normaliseDraftKind(value)
			},
		},
	}

	switch normaliseDraftKind(step.Kind) {
	case spec.KindDNS:
		fields = append(fields,
			builderFieldDef{
				Label:       "DNS Name",
				Placeholder: "example.org",
				Hint:        "Fully-qualified domain name to query.",
				Get:         func(m *builderModel) string { return m.currentStep().DNSName },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().DNSName = value },
			},
			builderFieldDef{
				Label:       "Record Type",
				Placeholder: "A",
				Options:     []string{"A", "AAAA", "CNAME", "MX", "NS", "TXT"},
				Hint:        "Cycles the DNS record type.",
				Get:         func(m *builderModel) string { return strings.ToUpper(strings.TrimSpace(m.currentStep().DNSType)) },
				Set: func(m *builderModel, value string) {
					m.currentStepPtr().DNSType = strings.ToUpper(strings.TrimSpace(value))
				},
			},
			builderFieldDef{
				Label:       "Resolver",
				Placeholder: "127.0.0.1:53",
				Hint:        "Optional DNS server override.",
				Get:         func(m *builderModel) string { return m.currentStep().DNSServer },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().DNSServer = value },
			},
			builderFieldDef{
				Label:       "Answer Has",
				Placeholder: "value1 | value2",
				Hint:        "Use | to require one or more answer substrings.",
				Get:         func(m *builderModel) string { return m.currentStep().DNSAnswerContains },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().DNSAnswerContains = value },
			},
			builderFieldDef{
				Label:       "Answer Lacks",
				Placeholder: "value1 | value2",
				Hint:        "Use | to forbid answer substrings.",
				Get:         func(m *builderModel) string { return m.currentStep().DNSAnswerAbsent },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().DNSAnswerAbsent = value },
			},
		)
	case spec.KindTLS:
		fields = append(fields,
			builderFieldDef{
				Label:       "Address",
				Placeholder: "example.org:443",
				Hint:        "Host[:port] for the TLS handshake.",
				Get:         func(m *builderModel) string { return m.currentStep().TLSAddress },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().TLSAddress = value },
			},
			builderFieldDef{
				Label:       "Server Name",
				Placeholder: "example.org",
				Hint:        "Optional SNI override.",
				Get:         func(m *builderModel) string { return m.currentStep().TLSServerName },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().TLSServerName = value },
			},
			builderFieldDef{
				Label:       "Min Days",
				Placeholder: "7",
				Hint:        "Minimum certificate days remaining.",
				Get:         func(m *builderModel) string { return m.currentStep().TLSDaysRemainingMin },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().TLSDaysRemainingMin = value },
			},
		)
	case spec.KindTCP:
		fields = append(fields,
			builderFieldDef{
				Label:       "Address",
				Placeholder: "example.org:443",
				Hint:        "Host[:port] for the TCP connectivity probe.",
				Get:         func(m *builderModel) string { return m.currentStep().TCPAddress },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().TCPAddress = value },
			},
		)
	default:
		fields = append(fields,
			builderFieldDef{
				Label:       "Method",
				Placeholder: "GET",
				Options:     []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
				Hint:        "Cycles common HTTP methods.",
				Get:         func(m *builderModel) string { return strings.ToUpper(strings.TrimSpace(m.currentStep().HTTPMethod)) },
				Set: func(m *builderModel, value string) {
					m.currentStepPtr().HTTPMethod = strings.ToUpper(strings.TrimSpace(value))
				},
			},
			builderFieldDef{
				Label:       "URL or Path",
				Placeholder: "https://example.org/ or /health",
				Hint:        "Full URL, bare host, or /path when base_url is set.",
				Get:         func(m *builderModel) string { return m.currentStep().HTTPTarget },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().HTTPTarget = value },
			},
			builderFieldDef{
				Label:       "Headers",
				Placeholder: "Host: example.test | User-Agent: smoke",
				Hint:        "Use | between request headers.",
				Get:         func(m *builderModel) string { return m.currentStep().HTTPHeaders },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().HTTPHeaders = value },
			},
			builderFieldDef{
				Label:       "Body",
				Placeholder: "{\"ok\":true}",
				Hint:        "Optional HTTP request body.",
				Get:         func(m *builderModel) string { return m.currentStep().HTTPBody },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().HTTPBody = value },
			},
			builderFieldDef{
				Label:       "Status",
				Placeholder: "200",
				Hint:        "Expected HTTP status code.",
				Get:         func(m *builderModel) string { return m.currentStep().HTTPStatus },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().HTTPStatus = value },
			},
			builderFieldDef{
				Label:       "Body Has",
				Placeholder: "needle1 | needle2",
				Hint:        "Use | to require body substrings.",
				Get:         func(m *builderModel) string { return m.currentStep().HTTPBodyContains },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().HTTPBodyContains = value },
			},
			builderFieldDef{
				Label:       "Body Lacks",
				Placeholder: "needle1 | needle2",
				Hint:        "Use | to forbid body substrings.",
				Get:         func(m *builderModel) string { return m.currentStep().HTTPBodyAbsent },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().HTTPBodyAbsent = value },
			},
			builderFieldDef{
				Label:       "Header Has",
				Placeholder: "Content-Type: text/html",
				Hint:        "Use | to require response header substrings.",
				Get:         func(m *builderModel) string { return m.currentStep().HTTPHeaderContains },
				Set:         func(m *builderModel, value string) { m.currentStepPtr().HTTPHeaderContains = value },
			},
		)
	}

	return fields
}

func (m builderModel) currentStep() builderStep {
	if len(m.steps) == 0 || m.selectedStep < 0 || m.selectedStep >= len(m.steps) {
		return builderStep{}
	}
	return m.steps[m.selectedStep]
}

func (m *builderModel) currentStepPtr() *builderStep {
	if len(m.steps) == 0 || m.selectedStep < 0 || m.selectedStep >= len(m.steps) {
		return nil
	}
	return &m.steps[m.selectedStep]
}

func (m *builderModel) clampSelection() {
	if len(m.steps) == 0 {
		m.selectedStep = 0
		m.selectedField = 0
		return
	}

	if m.selectedStep < 0 {
		m.selectedStep = 0
	}
	if m.selectedStep >= len(m.steps) {
		m.selectedStep = len(m.steps) - 1
	}

	fieldCount := len(m.fieldDefs())
	if fieldCount == 0 {
		m.selectedField = 0
		return
	}
	if m.selectedField < 0 {
		m.selectedField = 0
	}
	if m.selectedField >= fieldCount {
		m.selectedField = fieldCount - 1
	}
}

func (m builderModel) selectedFieldValue() string {
	fields := m.fieldDefs()
	if len(fields) == 0 || m.selectedField < 0 || m.selectedField >= len(fields) {
		return ""
	}
	return fields[m.selectedField].Get(&m)
}

func (m *builderModel) setSelectedFieldValue(value string) {
	fields := m.fieldDefs()
	if len(fields) == 0 || m.selectedField < 0 || m.selectedField >= len(fields) {
		return
	}
	fields[m.selectedField].Set(m, value)
	m.clampSelection()
}

func (m *builderModel) cycleSelectedField(delta int) bool {
	fields := m.fieldDefs()
	if len(fields) == 0 || m.selectedField < 0 || m.selectedField >= len(fields) {
		return false
	}

	options := fields[m.selectedField].Options
	if len(options) == 0 {
		return false
	}

	current := strings.TrimSpace(fields[m.selectedField].Get(m))
	current = strings.ToLower(current)
	index := 0
	for i, option := range options {
		if strings.ToLower(option) == current {
			index = i
			break
		}
	}

	index = (index + delta + len(options)) % len(options)
	fields[m.selectedField].Set(m, options[index])
	m.clampSelection()
	return true
}

func (m builderModel) currentFieldHint() string {
	fields := m.fieldDefs()
	if len(fields) == 0 || m.selectedField < 0 || m.selectedField >= len(fields) {
		return ""
	}
	return fields[m.selectedField].Hint
}

func draftPreviewText(m builderModel, buildErr error) string {
	lines := make([]string, 0, 64)
	if buildErr != nil {
		lines = append(lines, "# Not ready to run yet:")
		lines = append(lines, "# "+buildErr.Error())
		lines = append(lines, "")
	}

	name := strings.TrimSpace(m.specName)
	if name != "" {
		lines = append(lines, "name: "+strconv.Quote(name))
	}
	baseURL := strings.TrimSpace(m.baseURL)
	if baseURL != "" {
		lines = append(lines, "base_url: "+strconv.Quote(baseURL))
	}
	lines = append(lines, "steps:")

	for i, step := range m.steps {
		kind := normaliseDraftKind(step.Kind)
		name := strings.TrimSpace(step.Name)
		if name == "" {
			name = fmt.Sprintf("step-%d", i+1)
		}
		lines = append(lines, "  - name: "+strconv.Quote(name))
		lines = append(lines, "    kind: "+kind)

		expectLines := make([]string, 0, 8)

		switch kind {
		case spec.KindDNS:
			lines = append(lines, "    dns:")
			lines = append(lines, "      name: "+strconv.Quote(placeholderOrValue(step.DNSName, "example.org")))
			lines = append(lines, "      type: "+strings.ToUpper(placeholderOrValue(step.DNSType, "A")))
			if strings.TrimSpace(step.DNSServer) != "" {
				lines = append(lines, "      server: "+strconv.Quote(strings.TrimSpace(step.DNSServer)))
			}
			expectLines = appendBuilderList(expectLines, "      answer_contains:", splitBuilderList(step.DNSAnswerContains))
			expectLines = appendBuilderList(expectLines, "      answer_absent:", splitBuilderList(step.DNSAnswerAbsent))
		case spec.KindTLS:
			lines = append(lines, "    tls:")
			lines = append(lines, "      address: "+strconv.Quote(placeholderOrValue(step.TLSAddress, "example.org:443")))
			if strings.TrimSpace(step.TLSServerName) != "" {
				lines = append(lines, "      server_name: "+strconv.Quote(strings.TrimSpace(step.TLSServerName)))
			}
			if strings.TrimSpace(step.TLSDaysRemainingMin) != "" {
				expectLines = append(expectLines, "      days_remaining_at_least: "+strings.TrimSpace(step.TLSDaysRemainingMin))
			}
		case spec.KindTCP:
			lines = append(lines, "    tcp:")
			lines = append(lines, "      address: "+strconv.Quote(placeholderOrValue(step.TCPAddress, "example.org:443")))
		default:
			lines = append(lines, "    request:")
			lines = append(lines, "      method: "+strings.ToUpper(placeholderOrValue(step.HTTPMethod, "GET")))
			target := strings.TrimSpace(step.HTTPTarget)
			if strings.HasPrefix(target, "/") {
				lines = append(lines, "      path: "+strconv.Quote(target))
			} else {
				lines = append(lines, "      url: "+strconv.Quote(placeholderOrValue(target, "https://example.org/")))
			}
			headers := splitBuilderList(step.HTTPHeaders)
			if len(headers) > 0 {
				lines = append(lines, "      headers:")
				for _, header := range headers {
					name, value, ok := strings.Cut(header, ":")
					if !ok {
						lines = append(lines, "        "+strconv.Quote(strings.TrimSpace(header))+": \"\"")
						continue
					}
					lines = append(lines, "        "+strings.TrimSpace(name)+": "+strconv.Quote(strings.TrimSpace(value)))
				}
			}
			if strings.TrimSpace(step.HTTPBody) != "" {
				lines = append(lines, "      body: "+strconv.Quote(step.HTTPBody))
			}
			expectLines = append(expectLines, "      status: "+placeholderOrValue(strings.TrimSpace(step.HTTPStatus), "200"))
			expectLines = appendBuilderList(expectLines, "      body_contains:", splitBuilderList(step.HTTPBodyContains))
			expectLines = appendBuilderList(expectLines, "      body_absent:", splitBuilderList(step.HTTPBodyAbsent))
			expectLines = appendBuilderList(expectLines, "      header_contains:", splitBuilderList(step.HTTPHeaderContains))
		}

		if len(expectLines) > 0 {
			lines = append(lines, "    expect:")
			lines = append(lines, expectLines...)
		}
	}

	return strings.Join(lines, "\n")
}

func appendBuilderList(lines []string, header string, values []string) []string {
	if len(values) == 0 {
		return lines
	}
	lines = append(lines, header)
	for _, value := range values {
		lines = append(lines, "        - "+strconv.Quote(value))
	}
	return lines
}

func parseBuilderPositiveInt(raw string, label string, defaultValue int) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", label)
	}
	return parsed, nil
}

func parseBuilderOptionalPositiveInt(raw string, label string) (*int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := parseBuilderPositiveInt(value, label, 0)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseBuilderHeaders(raw string) (map[string]string, error) {
	values := splitBuilderList(raw)
	if len(values) == 0 {
		return nil, nil
	}

	headers := make(map[string]string, len(values))
	for _, value := range values {
		name, headerValue, ok := strings.Cut(value, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q: expected 'Name: value'", value)
		}
		name = strings.TrimSpace(name)
		headerValue = strings.TrimSpace(headerValue)
		if name == "" {
			return nil, fmt.Errorf("invalid header %q: header name is empty", value)
		}
		headers[name] = headerValue
	}
	return headers, nil
}

func splitBuilderList(raw string) []string {
	replacer := strings.NewReplacer("\r\n", "\n", "\r", "\n", "|", "\n")
	raw = replacer.Replace(raw)
	parts := strings.Split(raw, "\n")

	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		values = append(values, part)
	}
	return values
}

func normaliseBuilderTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("target is required")
	}
	if strings.HasPrefix(target, "/") {
		return target, nil
	}
	return adhoc.NormaliseTarget(target)
}

func placeholderOrValue(value string, placeholder string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return placeholder
	}
	return value
}

func normaliseDraftKind(kind string) string {
	kind = strings.TrimSpace(strings.ToLower(kind))
	switch kind {
	case spec.KindDNS, spec.KindTLS, spec.KindTCP:
		return kind
	default:
		return spec.KindHTTP
	}
}

func nextBuilderFocus(current builderFocus) builderFocus {
	switch current {
	case focusFields:
		return focusSteps
	default:
		return focusFields
	}
}

func previousBuilderFocus(current builderFocus) builderFocus {
	return nextBuilderFocus(current)
}

func (f builderFocus) String() string {
	switch f {
	case focusFields:
		return "fields"
	default:
		return "steps"
	}
}

func stepSummary(step builderStep) string {
	switch normaliseDraftKind(step.Kind) {
	case spec.KindDNS:
		recordType := strings.ToUpper(placeholderOrValue(step.DNSType, "A"))
		return placeholderOrValue(step.DNSName, "example.org") + " " + recordType
	case spec.KindTLS:
		return placeholderOrValue(step.TLSAddress, "example.org:443")
	case spec.KindTCP:
		return placeholderOrValue(step.TCPAddress, "example.org:443")
	default:
		return placeholderOrValue(step.HTTPTarget, "https://example.org/")
	}
}

func trimLastRune(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	return string(runes[:len(runes)-1])
}
