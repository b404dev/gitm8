package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/b404dev/gitm8/internal/config"
	"github.com/b404dev/gitm8/internal/git"
)

var setupLabels = []string{"Workspace directory", "Git name", "Git email", "Default branch", "Editor"}

type setupAuthFinishedMsg struct{ err error }

func newSetupInputs(runner git.Runner, cfg config.Config) []textinput.Model {
	values := []string{cfg.WorkspaceDir, runner.GlobalConfigValue(context.Background(), "user.name"), runner.GlobalConfigValue(context.Background(), "user.email"), cfg.DefaultBranch, cfg.Editor}
	inputs := make([]textinput.Model, len(values))
	for i, value := range values {
		input := textinput.New()
		input.Prompt = "> "
		input.CharLimit = 240
		input.SetValue(value)
		inputs[i] = input
	}
	return inputs
}

func (m Model) updateSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.setupStage {
	case "welcome":
		switch key {
		case "y", "enter":
			m.setupStage = "form"
			m.setupStep = 0
			m.setupInputs[0].Focus()
			return m, textinput.Blink
		case "n", "esc":
			if err := config.DismissFirstRunSetup(); err != nil {
				m.err = err
				return m, nil
			}
			return m.finishSetup("First-run setup dismissed.")
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case "form":
		if key == "esc" {
			m.setupInputs[m.setupStep].Blur()
			m.setupStage = "welcome"
			return m, nil
		}
		if key == "enter" {
			if strings.TrimSpace(m.setupInputs[m.setupStep].Value()) == "" {
				m.err = fmt.Errorf("%s cannot be empty", setupLabels[m.setupStep])
				return m, nil
			}
			if m.setupStep == 2 && !strings.Contains(m.setupInputs[2].Value(), "@") {
				m.err = fmt.Errorf("enter a valid Git email address")
				return m, nil
			}
			m.err = nil
			m.setupInputs[m.setupStep].Blur()
			if m.setupStep < len(m.setupInputs)-1 {
				m.setupStep++
				m.setupInputs[m.setupStep].Focus()
				return m, textinput.Blink
			}
			m.setupStage = "review"
			return m, nil
		}
		var cmd tea.Cmd
		m.setupInputs[m.setupStep], cmd = m.setupInputs[m.setupStep].Update(msg)
		return m, cmd
	case "review":
		switch key {
		case "y", "enter":
			m.setupStage = "saving"
			m.err = nil
			return m, m.saveSetup()
		case "b", "esc":
			m.setupStage = "form"
			m.setupStep = 0
			m.setupInputs[0].Focus()
			return m, textinput.Blink
		}
	case "auth":
		switch key {
		case "y", "enter":
			return m, runGitHubAuth()
		case "n", "esc":
			return m.finishSetup("Setup saved. GitHub authentication skipped.")
		}
	case "error":
		if key == "b" || key == "esc" {
			m.setupStage = "review"
			m.err = nil
		}
	}
	return m, nil
}

func (m Model) setupValues() config.SetupValues {
	return config.SetupValues{WorkspaceDir: expandSetupPath(m.setupInputs[0].Value()), Name: strings.TrimSpace(m.setupInputs[1].Value()), Email: strings.TrimSpace(m.setupInputs[2].Value()), DefaultBranch: strings.TrimSpace(m.setupInputs[3].Value()), Editor: strings.TrimSpace(m.setupInputs[4].Value())}
}

func expandSetupPath(path string) string {
	path = strings.TrimSpace(path)
	if home, err := os.UserHomeDir(); err == nil {
		if path == "~" {
			return home
		}
		if strings.HasPrefix(path, "~/") {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func (m Model) saveSetup() tea.Cmd {
	values := m.setupValues()
	return func() tea.Msg {
		if err := os.MkdirAll(values.WorkspaceDir, 0o755); err != nil {
			return setupFinishedMsg{err: err}
		}
		out, err := m.runner.ConfigureGlobalIdentityOutput(context.Background(), values.Name, values.Email, values.DefaultBranch)
		if err != nil {
			return setupFinishedMsg{output: out, err: err}
		}
		if err := config.SaveFirstRunSetup(values); err != nil {
			return setupFinishedMsg{output: out, err: err}
		}
		return setupFinishedMsg{output: out}
	}
}

func (m Model) handleSetupFinished(msg setupFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.setupStage = "error"
		return m, nil
	}
	m.gitOutput = msg.output
	m.config.WorkspaceDir = m.setupValues().WorkspaceDir
	m.config.DefaultBranch = m.setupValues().DefaultBranch
	m.config.Editor = m.setupValues().Editor
	m.setupStage = "auth"
	return m, nil
}

func runGitHubAuth() tea.Cmd {
	return func() tea.Msg {
		path, err := exec.LookPath("gh")
		if err != nil {
			return setupAuthFinishedMsg{err: fmt.Errorf("GitHub CLI is not installed or not on PATH")}
		}
		return tea.ExecProcess(exec.Command(path, "auth", "login", "-h", "github.com"), func(err error) tea.Msg { return setupAuthFinishedMsg{err: err} })()
	}
}

func (m Model) handleSetupAuthFinished(msg setupAuthFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.setupStage = "auth"
		return m, nil
	}
	return m.finishSetup("Setup saved and GitHub authentication completed.")
}

func (m Model) finishSetup(notice string) (tea.Model, tea.Cmd) {
	if !m.runner.IsRepository(context.Background()) {
		m.notice = notice
		return m.openWorkspace()
	}
	m.mode = "review"
	m.notice = notice
	m.splash = true
	m.splashFrame = 0
	return m, tickSplash()
}

func (m Model) setupView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Welcome to gitm8") + "\n\n")
	switch m.setupStage {
	case "welcome":
		b.WriteString("No GitHub workspace directory was found.\n")
		b.WriteString("Set up your workspace and Git identity now?\n\n")
		b.WriteString(mutedStyle.Render("y/enter: set up  n/esc: decline and remember  q: quit"))
	case "form":
		b.WriteString("These values are not saved until you confirm the review.\n\n")
		b.WriteString(titleStyle.Render(setupLabels[m.setupStep]) + "\n")
		b.WriteString(m.setupInputs[m.setupStep].View() + "\n\n")
		b.WriteString(mutedStyle.Render(fmt.Sprintf("step %d/%d  enter: next  esc: back", m.setupStep+1, len(m.setupInputs))))
	case "review", "saving", "error":
		values := m.setupValues()
		fmt.Fprintf(&b, "Review your configuration before saving:\n\n  Workspace:      %s\n  Git name:       %s\n  Git email:      %s\n  Default branch: %s\n  Editor:         %s\n\n", values.WorkspaceDir, values.Name, values.Email, values.DefaultBranch, values.Editor)
		if m.setupStage == "saving" {
			b.WriteString(keyStyle.Render("Saving configuration..."))
		} else if m.setupStage == "error" {
			b.WriteString(errorStyle.Render(m.err.Error()) + "\n" + mutedStyle.Render("b/esc: return to review"))
		} else {
			b.WriteString(mutedStyle.Render("y/enter: confirm and save  b/esc: edit"))
		}
	case "auth":
		b.WriteString("Configuration saved. Authenticate with GitHub now?\n\n")
		if m.err != nil {
			b.WriteString(errorStyle.Render(m.err.Error()) + "\n\n")
		}
		b.WriteString(mutedStyle.Render("y/enter: open gh auth login  n/esc: skip"))
	}
	cardWidth := clamp(m.width-8, 48, 76)
	card := panelStyle.Width(cardWidth).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}
