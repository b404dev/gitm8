package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/config"
	"github.com/b404dev/gitm8/internal/git"
)

func TestSetupRequiresReviewBeforeSaving(t *testing.T) {
	m := Model{
		mode:       "setup",
		setupStage: "welcome",
		setupInputs: newSetupInputs(git.NewRunner(t.TempDir()), config.Config{
			WorkspaceDir:  "/tmp/example-workspace",
			DefaultBranch: "main",
			Editor:        "vi",
		}),
	}

	next, _ := m.updateSetup(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = next.(Model)
	if m.setupStage != "form" {
		t.Fatalf("stage = %q, want form", m.setupStage)
	}

	values := []string{"/tmp/example-workspace", "Ada", "ada@example.com", "main", "vim"}
	for i, value := range values {
		m.setupInputs[i].SetValue(value)
		next, _ = m.updateSetup(tea.KeyMsg{Type: tea.KeyEnter})
		m = next.(Model)
	}
	if m.setupStage != "review" {
		t.Fatalf("stage = %q, want review", m.setupStage)
	}
	if !strings.Contains(m.setupView(), "Review your configuration before saving") {
		t.Fatal("review view does not show confirmation summary")
	}
}

func TestNewOutsideRepositoryOpensWorkspace(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "Github")
	m := New(git.NewRunner(t.TempDir()), config.Config{
		WorkspaceDir:      workspace,
		FirstRunDismissed: true,
		DefaultBranch:     "main",
		Editor:            "vi",
	})
	if m.mode != "workspace" || m.splash {
		t.Fatalf("mode = %q splash = %t, want workspace without splash", m.mode, m.splash)
	}
	if m.runner.IsRepository(context.Background()) {
		t.Fatal("test runner unexpectedly points at a repository")
	}
	m.ready, m.width, m.height = true, 100, 30
	if !strings.Contains(m.View(), "Projects") {
		t.Fatal("outside-repository startup did not render project picker")
	}
}

func TestWorkspaceIgnoresStaleRepositoryLoad(t *testing.T) {
	m := Model{mode: "workspace"}
	next, _ := m.Update(repoLoadedMsg{mode: "review", target: "repo"})
	if next.(Model).mode != "workspace" {
		t.Fatal("stale repository result closed workspace picker")
	}
}
