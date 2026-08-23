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
	contentWidth := max(40, m.width-4)
	header := brandHeader(contentWidth, "WORKSPACE", "Choose where you want to work")

	if m.projectAction != "" {
		var b strings.Builder
		label := "Clone GitHub repository"
		description := "Enter owner/repository and gitm8 will clone it into your workspace."
		if m.projectAction == "init" {
			label = "Create local repository"
			description = "Enter a folder name and gitm8 will initialize it with branch " + m.config.DefaultBranch + "."
		}
		b.WriteString(titleStyle.Render(label) + "\n")
		b.WriteString(mutedStyle.Render(description) + "\n\n")
		b.WriteString(m.projectInput.View() + "\n\n")
		b.WriteString(keyStyle.Render("enter") + mutedStyle.Render(" continue") + "    " + keyStyle.Render("esc") + mutedStyle.Render(" cancel"))
		if m.err != nil {
			b.WriteString("\n\n" + errorStyle.Render("! "+m.err.Error()))
		}
		dialog := panelStyle.Width(clamp(contentWidth-12, 44, 72)).Padding(1, 2).Render(b.String())
		bodyHeight := max(8, m.height-lipgloss.Height(header)-2)
		body := lipgloss.Place(contentWidth+2, bodyHeight, lipgloss.Center, lipgloss.Center, dialog)
		return lipgloss.JoinVertical(lipgloss.Left, header, body)
	}

	bodyHeight := max(8, m.height-lipgloss.Height(header)-lipgloss.Height(m.workspaceFooter())-1)
	listWidth := clamp(contentWidth*2/3, 34, 72)
	detailWidth := max(24, contentWidth-listWidth-3)
	if contentWidth < 76 {
		listWidth = contentWidth
		detailWidth = contentWidth
	}
	list := m.workspaceListPanel(listWidth, bodyHeight)
	detail := m.workspaceDetailPanel(detailWidth, bodyHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
	if contentWidth < 76 {
		body = list
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, m.workspaceFooter())
}

func (m Model) workspaceListPanel(width, height int) string {
	lines := []string{titleStyle.Render("Repositories") + "  " + keyStyle.Render(fmt.Sprintf("%d", len(m.projects))), mutedStyle.Render("↑/↓ navigate  enter open"), ""}
	if len(m.projects) == 0 {
		lines = append(lines, mutedStyle.Render("No repositories found."), "", "Clone a GitHub repository or create", "a fresh local project to get started.")
	} else {
		maxRows := max(1, height-6)
		start := clamp(m.projectCursor-maxRows+1, 0, max(0, len(m.projects)-maxRows))
		end := min(len(m.projects), start+maxRows)
		for i := start; i < end; i++ {
			rel, err := filepath.Rel(m.config.WorkspaceDir, m.projects[i])
			if err != nil {
				rel = m.projects[i]
			}
			marker, badge, style := "  ", mutedStyle.Render("git"), mutedStyle
			if i == m.projectCursor {
				marker, badge, style = keyStyle.Render("▸ "), keyStyle.Render("git"), activeStyle.Copy().Bold(true)
			}
			nameWidth := max(8, width-13)
			lines = append(lines, marker+style.Render(trimMiddle(rel, nameWidth))+"  "+badge)
		}
		if len(m.projects) > maxRows {
			lines = append(lines, "", mutedStyle.Render(fmt.Sprintf("showing %d-%d of %d", start+1, end, len(m.projects))))
		}
	}
	if m.err != nil {
		lines = append(lines, "", errorStyle.Render("! "+m.err.Error()))
	}
	return panelStyle.Width(width).Height(max(1, height-2)).Render(strings.Join(lines, "\n"))
}

func (m Model) workspaceDetailPanel(width, height int) string {
	var lines []string
	lines = append(lines, titleStyle.Render("Workspace"), "", keyStyle.Render("ROOT"), mutedStyle.Render(trimMiddle(m.config.WorkspaceDir, max(12, width-4))), "")
	if len(m.projects) > 0 {
		path := m.projects[clamp(m.projectCursor, 0, len(m.projects)-1)]
		rel, err := filepath.Rel(m.config.WorkspaceDir, path)
		if err != nil {
			rel = filepath.Base(path)
		}
		lines = append(lines, keyStyle.Render("SELECTED"), activeStyle.Copy().Bold(true).Render(trimMiddle(rel, max(12, width-4))), "", mutedStyle.Render("Ready to open"))
	} else {
		lines = append(lines, keyStyle.Render("STATUS"), mutedStyle.Render("Workspace is empty"))
	}
	lines = append(lines, "", titleStyle.Render("Quick actions"), keyStyle.Render("c")+"  Clone from GitHub", keyStyle.Render("n")+"  New local repository", keyStyle.Render("r")+"  Rescan workspace")
	return panelStyle.Width(width).Height(max(1, height-2)).Render(strings.Join(lines, "\n"))
}

func (m Model) workspaceFooter() string {
	items := []string{keyStyle.Render("[↑/↓]") + " choose", keyStyle.Render("[enter]") + " open", keyStyle.Render("[c]") + " clone", keyStyle.Render("[n]") + " create", keyStyle.Render("[r]") + " refresh", keyStyle.Render("[esc]") + " back", keyStyle.Render("[q]") + " quit"}
	return mutedStyle.Width(max(20, m.width)).Render(strings.Join(items, "  "))
}
