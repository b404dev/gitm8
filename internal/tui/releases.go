package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/git"
)

func newReleaseInputs() []textinput.Model {
	fields := []struct {
		placeholder string
		limit       int
	}{
		{"v1.0.0", 120}, {"Release title (defaults to tag)", 200}, {"Release notes (optional)", 2000},
	}
	inputs := make([]textinput.Model, len(fields))
	for i, field := range fields {
		inputs[i] = textinput.New()
		inputs[i].Prompt = "> "
		inputs[i].Placeholder = field.placeholder
		inputs[i].CharLimit = field.limit
	}
	return inputs
}

func loadReleases(runner git.Runner) tea.Cmd {
	return func() tea.Msg {
		releases, err := runner.Releases(context.Background())
		return releasesLoadedMsg{releases: releases, err: err}
	}
}

func createRelease(runner git.Runner, tag, title, notes string, draft, prerelease, generateNotes bool) tea.Cmd {
	return func() tea.Msg {
		out, err := runner.CreateRelease(context.Background(), tag, title, notes, draft, prerelease, generateNotes)
		return releaseCreatedMsg{output: out, err: err}
	}
}

func (m Model) updateReleases(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "review"
		return m, loadReview(m.runner, "")
	case "n":
		m.mode = "release-create"
		m.releaseInput = 0
		m.releaseDraft, m.releasePrerelease, m.releaseGenerateNotes = false, false, false
		for i := range m.releaseInputs {
			m.releaseInputs[i].SetValue("")
			m.releaseInputs[i].Blur()
		}
		m.releaseInputs[0].Focus()
		m.err, m.notice = nil, ""
		return m, nil
	case "r":
		return m, loadReleases(m.runner)
	case "up", "k":
		m.releaseCursor = clamp(m.releaseCursor-1, 0, max(0, len(m.releases)-1))
	case "down", "j":
		m.releaseCursor = clamp(m.releaseCursor+1, 0, max(0, len(m.releases)-1))
	}
	return m, nil
}

func (m Model) updateReleaseCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.loading {
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}
	switch key {
	case "esc":
		m.releaseInputs[m.releaseInput].Blur()
		m.mode = "releases"
		return m, nil
	case "tab", "down":
		m.releaseInputs[m.releaseInput].Blur()
		m.releaseInput = (m.releaseInput + 1) % len(m.releaseInputs)
		m.releaseInputs[m.releaseInput].Focus()
		return m, nil
	case "shift+tab", "up":
		m.releaseInputs[m.releaseInput].Blur()
		m.releaseInput = (m.releaseInput - 1 + len(m.releaseInputs)) % len(m.releaseInputs)
		m.releaseInputs[m.releaseInput].Focus()
		return m, nil
	case "ctrl+d":
		m.releaseDraft = !m.releaseDraft
		return m, nil
	case "ctrl+p":
		m.releasePrerelease = !m.releasePrerelease
		return m, nil
	case "ctrl+g":
		m.releaseGenerateNotes = !m.releaseGenerateNotes
		return m, nil
	case "enter":
		if strings.TrimSpace(m.releaseInputs[0].Value()) == "" {
			m.err = fmt.Errorf("a release tag is required")
			return m, nil
		}
		for i := range m.releaseInputs {
			m.releaseInputs[i].Blur()
		}
		m.loading = true
		m.gitOutput = "Creating GitHub release..."
		return m, tea.Batch(createRelease(m.runner, m.releaseInputs[0].Value(), m.releaseInputs[1].Value(), m.releaseInputs[2].Value(), m.releaseDraft, m.releasePrerelease, m.releaseGenerateNotes), m.spinner.Tick)
	}
	var cmd tea.Cmd
	m.err = nil
	m.releaseInputs[m.releaseInput], cmd = m.releaseInputs[m.releaseInput].Update(msg)
	return m, cmd
}

func (m Model) releasesView() string {
	var b strings.Builder
	b.WriteString("GitHub releases: n create, r refresh, esc return.\n\n")
	if len(m.releases) == 0 {
		b.WriteString("No releases found. Press n to create one.\n")
		return b.String()
	}
	for i, release := range m.releases {
		pointer := "  "
		if i == m.releaseCursor {
			pointer = "> "
		}
		state := "published"
		if release.Draft {
			state = "draft"
		} else if release.Prerelease {
			state = "prerelease"
		}
		name := release.Name
		if name == "" {
			name = release.Tag
		}
		date := release.Published
		if len(date) >= 10 {
			date = date[:10]
		}
		fmt.Fprintf(&b, "%s%-16s  %-12s  %-10s  %s\n", pointer, release.Tag, state, date, name)
	}
	return b.String()
}

func (m Model) releaseCreateView() string {
	labels := []string{"Tag", "Title", "Notes"}
	var b strings.Builder
	b.WriteString("Create GitHub release\n\n")
	for i, input := range m.releaseInputs {
		fmt.Fprintf(&b, "%s\n%s\n\n", labels[i], input.View())
	}
	mark := func(v bool) string {
		if v {
			return "[x]"
		}
		return "[ ]"
	}
	fmt.Fprintf(&b, "%s draft (ctrl+d)  %s prerelease (ctrl+p)  %s generated notes (ctrl+g)\n\n", mark(m.releaseDraft), mark(m.releasePrerelease), mark(m.releaseGenerateNotes))
	if m.loading {
		b.WriteString(keyStyle.Render("Creating GitHub release...") + "\n\n")
	} else if m.err != nil {
		b.WriteString(errorStyle.Render(m.err.Error()) + "\n\n")
	}
	b.WriteString("tab/shift+tab fields  enter create  esc cancel\n")
	return b.String()
}
