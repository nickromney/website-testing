package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	idx int
	res runner.Result
}

type model struct {
	spec    *spec.Spec
	runner  *runner.Runner
	current int
	results []runner.Result
	states  []status
	done    bool
	err     error
}

func Run(specDoc *spec.Spec, r *runner.Runner) error {
	m := model{
		spec:    specDoc,
		runner:  r,
		results: make([]runner.Result, len(specDoc.Steps)),
		states:  make([]status, len(specDoc.Steps)),
	}

	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	if len(m.spec.Steps) == 0 {
		m.done = true
		return nil
	}

	m.current = 0
	m.states[0] = statusRunning
	return runStepCmd(m.runner, m.spec, 0)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	case stepFinishedMsg:
		m.results[msg.idx] = msg.res
		if msg.res.Passed {
			m.states[msg.idx] = statusOK
		} else {
			m.states[msg.idx] = statusFail
		}

		m.current = msg.idx + 1
		if m.current >= len(m.spec.Steps) {
			m.done = true
			return m, nil
		}

		m.states[m.current] = statusRunning
		return m, runStepCmd(m.runner, m.spec, m.current)
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("smoke-go\n\n")

	for i, step := range m.spec.Steps {
		prefix := "[ ]"
		switch m.states[i] {
		case statusRunning:
			prefix = "[~]"
		case statusOK:
			prefix = "[OK]"
		case statusFail:
			prefix = "[FAIL]"
		}
		b.WriteString(fmt.Sprintf("%s %s\n", prefix, step.Name))
	}

	if m.done {
		b.WriteString("\nDone. Press q to quit.\n")
	} else {
		b.WriteString("\nRunning... (q to quit)\n")
	}
	return b.String()
}

func runStepCmd(r *runner.Runner, specDoc *spec.Spec, idx int) tea.Cmd {
	if idx < 0 || idx >= len(specDoc.Steps) {
		return nil
	}

	step := specDoc.Steps[idx]
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res := r.RunStep(ctx, step)
		return stepFinishedMsg{idx: idx, res: res}
	}
}
