package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/git"
)

func TestCommandPaletteFiltersAndMoves(t *testing.T) {
	m := Model{mode: "command-palette", commandInput: textinput.New()}
	m.commandInput.SetValue("release")
	commands := m.filteredCommands()
	if len(commands) != 1 || commands[0].key != "v" {
		t.Fatalf("filteredCommands() = %#v", commands)
	}
	next, _ := m.updateCommandPalette(tea.KeyMsg{Type: tea.KeyDown})
	if next.(Model).commandCursor != 0 {
		t.Fatal("single-result command palette moved beyond its result")
	}
}

func TestBrandHeaderUsesV3Identity(t *testing.T) {
	got := brandHeader(80, "THEMES", "Preview")
	if !strings.Contains(got, "GITM8") || !strings.Contains(got, "// THEMES") {
		t.Fatalf("brandHeader() = %q", got)
	}
}

func TestMouseWheelMovesReleaseSelection(t *testing.T) {
	m := Model{mode: "releases", releases: []git.Release{{Tag: "v1"}, {Tag: "v2"}}}
	next, _ := m.updateMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	if next.(Model).releaseCursor != 1 {
		t.Fatalf("mouse wheel left release cursor at %d", next.(Model).releaseCursor)
	}
}
