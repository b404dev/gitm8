package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/b404dev/gitm8/internal/git"
)

func loadProjects(root string) tea.Cmd {
	return func() tea.Msg {
		projects, err := git.WorkspaceRepositories(root)
		return projectsLoadedMsg{projects: projects, err: err}
	}
}

func (m Model) openWorkspace() (tea.Model, tea.Cmd) {
	m.mode = "workspace"
	m.projectAction = ""
	m.projectInput.Blur()
	m.err = nil
	m.notice = ""
	return m, loadProjects(m.config.WorkspaceDir)
}

func (m Model) updateWorkspace(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.projectAction != "" {
		if key == "esc" {
			m.projectAction = ""
			m.projectInput.Blur()
			m.err = nil
			return m, nil
		}
		if key == "enter" {
			value := strings.TrimSpace(m.projectInput.Value())
			if value == "" {
				m.err = fmt.Errorf("a value is required")
				return m, nil
			}
			action := m.projectAction
			m.projectInput.Blur()
			return m, runProjectCreate(action, m.config.WorkspaceDir, value, m.config.DefaultBranch)
		}
		var cmd tea.Cmd
		m.projectInput, cmd = m.projectInput.Update(msg)
		return m, cmd
	}

	switch key {
	case "up", "k":
		m.projectCursor = clamp(m.projectCursor-1, 0, max(0, len(m.projects)-1))
	case "down", "j":
		m.projectCursor = clamp(m.projectCursor+1, 0, max(0, len(m.projects)-1))
	case "enter":
		if len(m.projects) == 0 {
			m.err = fmt.Errorf("no repositories found; clone or create one")
			return m, nil
		}
		path := m.projects[m.projectCursor]
		m.runner = git.NewRunner(path)
		m.mode = "review"
		m.err = nil
		return m, loadDefault(m.runner)
	case "c", "n":
		m.projectAction = map[string]string{"c": "clone", "n": "init"}[key]
		m.projectInput.SetValue("")
		if key == "c" {
			m.projectInput.Placeholder = "owner/repository"
		} else {
			m.projectInput.Placeholder = "project-name"
		}
		m.projectInput.Focus()
		m.err = nil
		return m, nil
	case "r":
		return m, loadProjects(m.config.WorkspaceDir)
	case "esc":
		if m.runner.IsRepository(context.Background()) {
			m.mode = "review"
			return m, loadDefault(m.runner)
		}
		return m, tea.Quit
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func runProjectCreate(action, root, value, branch string) tea.Cmd {
	return func() tea.Msg {
		var path, output string
		var err error
		if action == "clone" {
			path, output, err = git.CloneWorkspaceRepository(context.Background(), root, value)
		} else {
			path, output, err = git.InitWorkspaceRepository(context.Background(), root, value, branch)
		}
		return projectOpenedMsg{path: path, output: output, err: err}
	}
}

func (m Model) handleProjectOpened(msg projectOpenedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.projectInput.Focus()
		return m, nil
	}
	m.runner = git.NewRunner(msg.path)
	m.mode = "review"
	m.projectAction = ""
	m.gitOutput = strings.TrimSpace(msg.output)
	if m.gitOutput == "" {
		m.gitOutput = "Opened " + msg.path
	}
	return m, loadDefault(m.runner)
}

func (m Model) workspaceView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Projects") + "\n")
	b.WriteString(mutedStyle.Render(m.config.WorkspaceDir) + "\n\n")
	if m.projectAction != "" {
		label := "Clone GitHub repository"
		if m.projectAction == "init" {
			label = "Create local repository"
		}
		b.WriteString(titleStyle.Render(label) + "\n" + m.projectInput.View() + "\n\n")
		b.WriteString(mutedStyle.Render("enter: continue  esc: cancel"))
	} else if len(m.projects) == 0 {
		b.WriteString("No Git repositories found in this workspace.\n\n")
		b.WriteString(mutedStyle.Render("c: clone from GitHub  n: create local  r: refresh  q: quit"))
	} else {
		maxRows := max(1, m.height-12)
		start := clamp(m.projectCursor-maxRows+1, 0, max(0, len(m.projects)-maxRows))
		end := min(len(m.projects), start+maxRows)
		for i := start; i < end; i++ {
			rel, err := filepath.Rel(m.config.WorkspaceDir, m.projects[i])
			if err != nil {
				rel = m.projects[i]
			}
			prefix := "  "
			style := mutedStyle
			if i == m.projectCursor {
				prefix = "> "
				style = activeStyle
			}
			b.WriteString(style.Render(prefix+rel) + "\n")
		}
		b.WriteString("\n" + mutedStyle.Render("↑/↓: choose  enter: open  c: clone  n: create  r: refresh  esc: return  q: quit"))
	}
	if m.err != nil {
		b.WriteString("\n\n" + errorStyle.Render(m.err.Error()))
	}
	cardWidth := clamp(m.width-8, 48, 84)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panelStyle.Width(cardWidth).Render(b.String()))
}
