package tui

import (
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
