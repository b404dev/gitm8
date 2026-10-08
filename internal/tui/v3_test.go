package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

func TestThemeSwatchPreviewsSemanticPalette(t *testing.T) {
	if got := lipgloss.Width(themeSwatch("catppuccin")); got != 10 {
		t.Fatalf("themeSwatch() width = %d, want 10", got)
	}
	if got := themeSwatch("does-not-exist"); got != "" {
		t.Fatalf("unknown themeSwatch() = %q, want empty", got)
	}
}

func TestTopBarSeparatesIdentityAndRepositoryHealth(t *testing.T) {
	m := Model{width: 120, mode: "review", info: git.RepoInfo{Repo: "gitm8", Branch: "feature/polish", Upstream: "origin/feature/polish", Staged: 2, Unstaged: 3, Ahead: 1, Behind: 4}}
	got := m.topBar()
	for _, want := range []string{"GITM8", "CODE REVIEW", "gitm8", "feature/polish", "5 CHANGES", "2 staged", "3 working", "↑ 1", "↓ 4"} {
		if !strings.Contains(got, want) {
			t.Fatalf("topBar() = %q, want %q", got, want)
		}
	}
}

func TestMouseWheelMovesReleaseSelection(t *testing.T) {
	m := Model{mode: "releases", releases: []git.Release{{Tag: "v1"}, {Tag: "v2"}}}
	next, _ := m.updateMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	if next.(Model).releaseCursor != 1 {
		t.Fatalf("mouse wheel left release cursor at %d", next.(Model).releaseCursor)
	}
}

func TestBrandedHelpFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 30}, {140, 40}} {
		m := Model{width: size[0], height: size[1], helpInput: textinput.New()}
		opened, _ := m.openHelpSelector()
		m = opened.(Model)
		got := m.helpScreenView()
		if lipgloss.Width(got) > size[0] || lipgloss.Height(got) > size[1] {
			t.Fatalf("help at %dx%d rendered %dx%d", size[0], size[1], lipgloss.Width(got), lipgloss.Height(got))
		}
		if !strings.Contains(got, "ALL KEYS") || !strings.Contains(got, "GITM8") {
			t.Fatal("help is missing the branded header")
		}
	}
}
