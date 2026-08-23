package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/git"
)

func TestReleasesViewShowsStates(t *testing.T) {
	m := Model{width: 100, height: 30, ready: true, mode: "releases", releases: []git.Release{
		{Tag: "v1.0.0", Name: "First", Published: "2026-08-23T10:00:00Z"},
		{Tag: "v2.0.0-rc1", Prerelease: true},
	}}
	got := m.releasesScreenView()
	if !strings.Contains(got, "v1.0.0") || !strings.Contains(got, "published") || !strings.Contains(got, "prerelease") {
		t.Fatalf("releasesView() = %q", got)
	}
}

func TestReleaseArrowKeysMoveVisibleSelection(t *testing.T) {
	m := Model{width: 100, height: 30, ready: true, mode: "releases", releases: []git.Release{{Tag: "v1"}, {Tag: "v2"}}}
	before := m.releasesScreenView()
	next, _ := m.updateReleases(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	after := m.releasesScreenView()
	if m.releaseCursor != 1 || before == after {
		t.Fatalf("down key left cursor=%d or did not redraw selection", m.releaseCursor)
	}
}

func TestEnterLoadsSelectedReleaseDetails(t *testing.T) {
	m := Model{mode: "releases", releases: []git.Release{{Tag: "v1"}, {Tag: "v2"}}, releaseCursor: 1}
	next, cmd := m.updateReleases(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.mode != "release-detail" || cmd == nil {
		t.Fatalf("enter produced mode=%q cmd=%v", m.mode, cmd)
	}
}

func TestReleaseCreateRequiresTag(t *testing.T) {
	m := Model{mode: "release-create", releaseInputs: newReleaseInputs()}
	next, _ := m.updateReleaseCreate(tea.KeyMsg{Type: tea.KeyEnter})
	if next.(Model).err == nil {
		t.Fatal("empty release tag was accepted")
	}
}
