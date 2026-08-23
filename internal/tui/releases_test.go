package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textarea"
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
	m := Model{mode: "release-create", releaseInputs: newReleaseInputs(), releaseNotes: textarea.New(), releaseProgress: progress.New()}
	next, _ := m.updateReleaseCreate(tea.KeyMsg{Type: tea.KeyEnter})
	if next.(Model).err == nil {
		t.Fatal("empty release tag was accepted")
	}
}

func TestSuggestNextReleaseTag(t *testing.T) {
	if got := suggestNextReleaseTag("v2.7.9"); got != "v2.7.10" {
		t.Fatalf("suggestNextReleaseTag() = %q, want v2.7.10", got)
	}
}

func TestReleaseCreateAdvancesToReviewBeforePublishing(t *testing.T) {
	m := Model{mode: "release-create", releaseInputs: newReleaseInputs(), releaseNotes: textarea.New(), releaseProgress: progress.New()}
	m.releaseInputs[0].SetValue("v3.0.0")
	for step := 1; step <= 4; step++ {
		key := tea.KeyMsg{Type: tea.KeyEnter}
		if m.releaseStep == 2 {
			key = tea.KeyMsg{Type: tea.KeyTab}
		}
		next, cmd := m.updateReleaseCreate(key)
		m = next.(Model)
		if cmd == nil || m.releaseStep != step {
			t.Fatalf("enter at step %d produced step=%d cmd=%v", step-1, m.releaseStep, cmd)
		}
	}
}
