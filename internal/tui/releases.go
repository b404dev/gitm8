package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

func loadReleaseDetail(runner git.Runner, tag string) tea.Cmd {
	return func() tea.Msg {
		detail, err := runner.ReleaseDetails(context.Background(), tag)
		return releaseDetailLoadedMsg{detail: detail, err: err}
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
	case "h":
		m.helpReturn = "releases"
		m.mode = "help"
		m.review.SetContent(m.helpView())
		m.review.GotoTop()
		return m, nil
	case "enter":
		if len(m.releases) == 0 {
			return m, nil
		}
		m.mode = "release-detail"
		m.err = nil
		m.review.SetContent("Loading release details...\n")
		m.review.GotoTop()
		return m, loadReleaseDetail(m.runner, m.releases[m.releaseCursor].Tag)
	case "up", "k":
		m.releaseCursor = clamp(m.releaseCursor-1, 0, max(0, len(m.releases)-1))
	case "down", "j":
		m.releaseCursor = clamp(m.releaseCursor+1, 0, max(0, len(m.releases)-1))
	}
	return m, nil
}

func (m Model) updateReleaseDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "releases"
		m.err = nil
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	case "h":
		m.helpReturn = "release-detail"
		m.mode = "help"
		m.review.SetContent(m.helpView())
		m.review.GotoTop()
		return m, nil
	}
	return m.updateViewportKey(msg)
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

func (m Model) releasesScreenView() string {
	contentWidth := max(40, m.width-4)
	header := panelStyle.Width(contentWidth).Render(strings.Join([]string{
		titleStyle.Render("gitm8") + "  " + keyStyle.Render("RELEASES"),
		mutedStyle.Render("Publish and inspect GitHub releases"),
	}, "\n"))
	if m.mode == "release-create" {
		bodyHeight := max(12, m.height-lipgloss.Height(header)-2)
		card := m.releaseCreateCard(clamp(contentWidth-10, 44, 78))
		body := lipgloss.Place(contentWidth+2, bodyHeight, lipgloss.Center, lipgloss.Center, card)
		return lipgloss.JoinVertical(lipgloss.Left, header, body)
	}
	if m.mode == "release-detail" {
		return m.releaseDetailScreen(header, contentWidth)
	}
	bodyHeight := max(8, m.height-lipgloss.Height(header)-lipgloss.Height(m.releasesFooter())-1)
	listWidth := clamp(contentWidth*2/3, 36, 74)
	detailWidth := max(24, contentWidth-listWidth-3)
	list := m.releasesListPanel(listWidth, bodyHeight)
	detail := m.releaseDetailPanel(detailWidth, bodyHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
	if contentWidth < 76 {
		body = m.releasesListPanel(contentWidth, bodyHeight)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, m.releasesFooter())
}

func (m Model) releaseDetailScreen(header string, contentWidth int) string {
	footer := mutedStyle.Width(max(20, m.width)).Render(strings.Join([]string{keyStyle.Render("[j/k]") + " scroll", keyStyle.Render("[pgup/pgdn]") + " page", keyStyle.Render("[g/G]") + " top/bottom", keyStyle.Render("[esc]") + " releases"}, "  "))
	bodyHeight := max(8, m.height-lipgloss.Height(header)-lipgloss.Height(footer)-1)
	m.review.Width = max(20, contentWidth-4)
	m.review.Height = max(1, bodyHeight-4)
	panel := panelStyle.Width(contentWidth).Height(max(1, bodyHeight-2)).Render(titleStyle.Render("Release details") + "\n" + m.review.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, panel, footer)
}

func releaseDetailContent(detail git.ReleaseDetail) string {
	name := detail.Name
	if name == "" {
		name = detail.Tag
	}
	date := detail.Published
	if len(date) >= 10 {
		date = date[:10]
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(name) + "\n")
	fmt.Fprintf(&b, "%s  %s  %s\n", keyStyle.Render(detail.Tag), releaseStateStyle(detail.Release).Render(releaseState(detail.Release)), mutedStyle.Render(date))
	if detail.Author != "" {
		fmt.Fprintf(&b, "Author: %s\n", detail.Author)
	}
	if detail.TargetCommitish != "" {
		fmt.Fprintf(&b, "Target: %s\n", detail.TargetCommitish)
	}
	if detail.URL != "" {
		fmt.Fprintf(&b, "URL: %s\n", detail.URL)
	}
	b.WriteString("\n" + titleStyle.Render("Release notes") + "\n\n")
	if strings.TrimSpace(detail.Body) == "" {
		b.WriteString(mutedStyle.Render("No release notes."))
	} else {
		b.WriteString(detail.Body)
	}
	b.WriteString("\n\n" + titleStyle.Render(fmt.Sprintf("Assets (%d)", len(detail.Assets))) + "\n")
	if len(detail.Assets) == 0 {
		b.WriteString("\n" + mutedStyle.Render("No assets attached."))
	}
	for _, asset := range detail.Assets {
		fmt.Fprintf(&b, "\n%s  %s  %s", keyStyle.Render(asset.Name), humanBytes(asset.Size), mutedStyle.Render(asset.ContentType))
	}
	b.WriteString("\n")
	return b.String()
}

func humanBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func (m Model) releasesListPanel(width, height int) string {
	lines := []string{titleStyle.Render("Releases") + "  " + keyStyle.Render(fmt.Sprintf("%d", len(m.releases))), mutedStyle.Render("↑/↓ navigate  enter inspect  n create"), ""}
	if len(m.releases) == 0 {
		lines = append(lines, mutedStyle.Render("No releases found."), "", "Press n to publish your first release.")
	} else {
		maxRows := max(1, height-6)
		start := clamp(m.releaseCursor-maxRows+1, 0, max(0, len(m.releases)-maxRows))
		end := min(len(m.releases), start+maxRows)
		for i := start; i < end; i++ {
			release := m.releases[i]
			marker, style := "  ", mutedStyle
			if i == m.releaseCursor {
				marker, style = keyStyle.Render("▸ "), activeStyle.Copy().Bold(true)
			}
			state := releaseState(release)
			lines = append(lines, marker+style.Render(trimMiddle(release.Tag, max(8, width-19)))+"  "+releaseStateStyle(release).Render(state))
		}
		if len(m.releases) > maxRows {
			lines = append(lines, "", mutedStyle.Render(fmt.Sprintf("showing %d-%d of %d", start+1, end, len(m.releases))))
		}
	}
	if m.err != nil {
		lines = append(lines, "", errorStyle.Render("! "+m.err.Error()))
	}
	return panelStyle.Width(width).Height(max(1, height-2)).Render(strings.Join(lines, "\n"))
}

func (m Model) releaseDetailPanel(width, height int) string {
	lines := []string{titleStyle.Render("Release details"), ""}
	if len(m.releases) == 0 {
		lines = append(lines, keyStyle.Render("STATUS"), mutedStyle.Render("Nothing published yet"), "", titleStyle.Render("Quick actions"), keyStyle.Render("n")+"  New release", keyStyle.Render("r")+"  Refresh")
	} else {
		release := m.releases[clamp(m.releaseCursor, 0, len(m.releases)-1)]
		name := release.Name
		if name == "" {
			name = release.Tag
		}
		date := release.Published
		if len(date) >= 10 {
			date = date[:10]
		}
		if date == "" {
			date = "Not published"
		}
		lines = append(lines,
			keyStyle.Render("TAG"), activeStyle.Copy().Bold(true).Render(trimMiddle(release.Tag, max(12, width-4))), "",
			keyStyle.Render("TITLE"), trimMiddle(name, max(12, width-4)), "",
			keyStyle.Render("STATUS"), releaseStateStyle(release).Render(releaseState(release)), "",
			keyStyle.Render("DATE"), mutedStyle.Render(date),
		)
	}
	return panelStyle.Width(width).Height(max(1, height-2)).Render(strings.Join(lines, "\n"))
}

func releaseState(release git.Release) string {
	if release.Draft {
		return "draft"
	}
	if release.Prerelease {
		return "prerelease"
	}
	return "published"
}

func releaseStateStyle(release git.Release) lipgloss.Style {
	if release.Draft {
		return mutedStyle
	}
	if release.Prerelease {
		return keyStyle
	}
	return activeStyle
}

func (m Model) releaseCreateCard(width int) string {
	labels := []string{"Tag", "Title", "Notes"}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Create GitHub release") + "\n")
	b.WriteString(mutedStyle.Render("Use an existing tag or enter a new one.") + "\n\n")
	for i, input := range m.releaseInputs {
		label := mutedStyle.Render(labels[i])
		if i == m.releaseInput {
			label = keyStyle.Render("▸ " + labels[i])
		}
		fmt.Fprintf(&b, "%s\n%s\n\n", label, input.View())
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
	b.WriteString(keyStyle.Render("enter") + mutedStyle.Render(" create") + "    " + keyStyle.Render("esc") + mutedStyle.Render(" cancel"))
	return panelStyle.Width(width).Padding(1, 2).Render(b.String())
}

func (m Model) releasesFooter() string {
	items := []string{keyStyle.Render("[h]") + " all keys", keyStyle.Render("[↑/↓]") + " choose", keyStyle.Render("[enter]") + " details", keyStyle.Render("[n]") + " create", keyStyle.Render("[esc]") + " back", keyStyle.Render("[q]") + " quit"}
	return mutedStyle.Width(max(20, m.width)).Render(strings.Join(items, "  "))
}
