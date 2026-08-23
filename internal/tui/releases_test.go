package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/git"
)

func TestReleasesViewShowsStates(t *testing.T) {
	m := Model{releases: []git.Release{
		{Tag: "v1.0.0", Name: "First", Published: "2026-08-23T10:00:00Z"},
		{Tag: "v2.0.0-rc1", Prerelease: true},
	}}
	got := m.releasesView()
	if !strings.Contains(got, "v1.0.0") || !strings.Contains(got, "published") || !strings.Contains(got, "prerelease") {
		t.Fatalf("releasesView() = %q", got)
	}
}

func TestReleaseCreateRequiresTag(t *testing.T) {
	m := Model{mode: "release-create", releaseInputs: newReleaseInputs()}
	next, _ := m.updateReleaseCreate(tea.KeyMsg{Type: tea.KeyEnter})
	if next.(Model).err == nil {
		t.Fatal("empty release tag was accepted")
	}
}
