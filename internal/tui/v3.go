package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/b404dev/gitm8/internal/config"
)

type paletteCommand struct {
	label       string
	description string
	key         string
}

var paletteCommands = []paletteCommand{
	{"Review repository", "Return to the code review", "0"},
	{"Changed files", "Focus the repository dashboard", "0"},
	{"Branches", "Switch, create, or delete branches", "b"},
	{"Releases", "Inspect and publish GitHub releases", "v"},
	{"Pull request", "Create a pull request", "r"},
	{"Stashes", "Inspect and apply stashes", "t"},
	{"Conflicts", "Resolve unmerged files", "C"},
	{"Commit log", "Browse recent history", "l"},
	{"Themes", "Preview and select a visual theme", "T"},
	{"Workspace", "Switch projects", "w"},
	{"Keyboard reference", "See every key binding", "h"},
}

func (m Model) openCommandPalette() (tea.Model, tea.Cmd) {
	m.returnMode = m.mode
	m.mode = "command-palette"
	m.commandCursor = 0
	m.commandInput.SetValue("")
	m.commandInput.Focus()
	return m, nil
}

func (m Model) filteredCommands() []paletteCommand {
	query := strings.ToLower(strings.TrimSpace(m.commandInput.Value()))
	if query == "" {
		return paletteCommands
	}
	var result []paletteCommand
	for _, command := range paletteCommands {
		haystack := strings.ToLower(command.label + " " + command.description)
		if strings.Contains(haystack, query) {
			result = append(result, command)
		}
	}
	return result
}

func (m Model) updateCommandPalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	commands := m.filteredCommands()
	switch msg.String() {
	case "esc":
		m.commandInput.Blur()
		m.mode = m.returnMode
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "up", "ctrl+k":
		m.commandCursor = clamp(m.commandCursor-1, 0, max(0, len(commands)-1))
		return m, nil
	case "down", "ctrl+j":
		m.commandCursor = clamp(m.commandCursor+1, 0, max(0, len(commands)-1))
		return m, nil
	case "enter":
		if len(commands) == 0 {
			return m, nil
		}
		command := commands[clamp(m.commandCursor, 0, len(commands)-1)]
		m.commandInput.Blur()
		m.mode = "review"
		return m.updateDashboardKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(command.key)})
	}
	var cmd tea.Cmd
	m.commandInput, cmd = m.commandInput.Update(msg)
	m.commandCursor = clamp(m.commandCursor, 0, max(0, len(m.filteredCommands())-1))
	return m, cmd
}

func (m Model) commandPaletteView() string {
	width := clamp(m.width-12, 48, 82)
	commands := m.filteredCommands()
	var lines []string
	lines = append(lines, titleStyle.Render("COMMAND PALETTE"), mutedStyle.Render("Jump anywhere without memorising a shortcut"), "", m.commandInput.View(), "")
	maxRows := max(3, min(10, m.height-12))
	start := clamp(m.commandCursor-maxRows+1, 0, max(0, len(commands)-maxRows))
	end := min(len(commands), start+maxRows)
	for i := start; i < end; i++ {
		command := commands[i]
		line := fmt.Sprintf("  %-24s %s", command.label, command.description)
		if i == m.commandCursor {
			line = selectedStyle.Width(max(20, width-6)).Render("▸ " + strings.TrimSpace(line))
		}
		lines = append(lines, line)
	}
	if len(commands) == 0 {
		lines = append(lines, mutedStyle.Render("No matching commands"))
	}
	lines = append(lines, "", mutedStyle.Render("↑/↓ choose  enter open  esc close"))
	card := panelStyle.Width(width).Padding(1, 2).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(max(40, m.width), max(16, m.height), lipgloss.Center, lipgloss.Center, card)
}

func (m Model) openThemes() (tea.Model, tea.Cmd) {
	m.returnMode = m.mode
	m.mode = "themes"
	names := themeNames()
	for i, name := range names {
		if name == m.config.Theme {
			m.themeCursor = i
		}
	}
	return m, nil
}

func (m Model) updateThemes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	names := themeNames()
	switch msg.String() {
	case "esc":
		applyTheme(m.config.Theme)
		m.mode = m.returnMode
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.themeCursor = clamp(m.themeCursor-1, 0, len(names)-1)
	case "down", "j":
		m.themeCursor = clamp(m.themeCursor+1, 0, len(names)-1)
	case "enter":
		name := names[m.themeCursor]
		if err := config.SaveTheme(name); err != nil {
			m.toast, m.toastError = "Could not save theme: "+err.Error(), true
			return m, nil
		}
		m.config.Theme = name
		m.toast, m.toastError = "Theme saved: "+name, false
		m.mode = m.returnMode
		return m, nil
	}
	applyTheme(names[m.themeCursor])
	return m, nil
}

func (m Model) themesView() string {
	names := themeNames()
	contentWidth := max(40, m.width-4)
	header := brandHeader(contentWidth, "THEMES", "Preview live · enter to make it yours")
	bodyHeight := max(8, m.height-lipgloss.Height(header)-2)
	width := clamp(contentWidth-12, 42, 68)
	maxRows := max(3, bodyHeight-7)
	start := clamp(m.themeCursor-maxRows+1, 0, max(0, len(names)-maxRows))
	end := min(len(names), start+maxRows)
	lines := []string{titleStyle.Render("Choose your gitm8 look"), mutedStyle.Render(fmt.Sprintf("%d built-in themes", len(names))), ""}
	for i := start; i < end; i++ {
		line := "  " + names[i]
		if names[i] == m.config.Theme {
			line += "  ✓ current"
		}
		if i == m.themeCursor {
			line = selectedStyle.Width(max(20, width-6)).Render("▸ " + strings.TrimSpace(line))
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", keyStyle.Render("enter")+" save    "+keyStyle.Render("esc")+" cancel")
	card := panelStyle.Width(width).Padding(1, 2).Render(strings.Join(lines, "\n"))
	body := lipgloss.Place(contentWidth+2, bodyHeight, lipgloss.Center, lipgloss.Center, card)
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func brandHeader(width int, section, subtitle string) string {
	return panelStyle.Width(width).Render(titleStyle.Render("GITM8") + "  " + keyStyle.Render("// "+section) + "\n" + mutedStyle.Render(subtitle))
}

// updateMouse adds trackpad navigation while preserving keyboard-first flows.
func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	event := tea.MouseEvent(msg)
	switch event.Button {
	case tea.MouseButtonWheelUp:
		return m.updateKey(tea.KeyMsg{Type: tea.KeyUp})
	case tea.MouseButtonWheelDown:
		return m.updateKey(tea.KeyMsg{Type: tea.KeyDown})
	case tea.MouseButtonLeft:
		if event.Action != tea.MouseActionPress {
			return m, nil
		}
		if m.mode == "releases" {
			row := event.Y - 8
			if row >= 0 {
				maxRows := max(1, m.height-10)
				start := clamp(m.releaseCursor-maxRows+1, 0, max(0, len(m.releases)-maxRows))
				index := start + row
				if index < len(m.releases) {
					m.releaseCursor = index
				}
			}
			return m, nil
		}
		if (m.mode == "review" || m.mode == "preview") && m.width >= 76 && event.X <= m.filesWidth()+2 {
			row := event.Y - lipgloss.Height(m.header()) - 3
			files := m.filteredFiles()
			index := m.fileOffset + row
			if row >= 0 && index < len(files) {
				m.fileCursor = index
				m.target = files[index].Path
				m.mode = "preview"
				return m, loadPreview(m.runner, m.target)
			}
		}
	}
	return m, nil
}
