package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// shortcutDelegate gives the searchable list the same visual language as the
// dashboard, with independent emphasis for keys, actions, and visibility.
type shortcutDelegate struct{}

func (shortcutDelegate) Height() int                         { return 1 }
func (shortcutDelegate) Spacing() int                        { return 0 }
func (shortcutDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (shortcutDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(helpListItem)
	if !ok {
		return
	}
	width := max(1, m.Width())
	marker := mutedStyle.Render("○")
	if i.selected {
		marker = keyStyle.Render("●")
	}
	name := mainFooterLabel(i.key.id)
	if name == "" {
		name = i.key.id
	}
	name = strings.ToUpper(name[:1]) + name[1:]
	pointer := "  "
	labelStyle := lipgloss.NewStyle()
	if index == m.Index() {
		pointer = keyStyle.Render("▸ ")
		labelStyle = activeStyle.Copy().Bold(true)
	}
	row := pointer + marker + "  " + keyStyle.Width(17).Render(i.key.key) + labelStyle.Render(name)
	if width >= 78 {
		row = pointer + marker + "  " + keyStyle.Width(17).Render(i.key.key) + labelStyle.Width(20).Render(name) + mutedStyle.Render(i.key.description)
	}
	fmt.Fprint(w, ansi.Truncate(row, width, "…"))
}

func (m Model) shortcutDetail(width int) string {
	if !m.helpListReady {
		return ""
	}
	item, ok := m.helpList.SelectedItem().(helpListItem)
	if !ok {
		return mutedStyle.Render("No matching shortcuts. Try a different search.")
	}
	state := "○ hidden from footer"
	if item.selected {
		state = "● shown in footer"
	}
	return keyStyle.Render(item.key.key) + "  " + mutedStyle.Render(state) + "\n" + lipgloss.NewStyle().Width(width).Render(item.key.description)
}
