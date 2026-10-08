package tui

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/b404dev/gitm8/internal/git"
)

// Main Screen Layout

// View renders the complete terminal frame for the current model state.
func (m Model) View() string {
	if !m.ready {
		return "Loading gitm8..."
	}
	if m.mode == "setup" {
		return m.setupView()
	}
	if m.mode == "releases" || m.mode == "release-create" || m.mode == "release-detail" {
		return m.releasesScreenView()
	}
	if m.mode == "help" {
		return m.helpScreenView()
	}
	if m.mode == "command-palette" {
		return m.commandPaletteView()
	}
	if m.mode == "themes" {
		return m.themesView()
	}
	if m.mode == "preview" && m.readerFocus {
		return m.focusedReaderView()
	}
	if m.splash {
		return m.splashView()
	}
	if m.mode == "workspace" {
		return m.workspaceView()
	}

	header := m.header()
	footer := m.footer()
	feedback := m.feedbackBar()
	bodyHeight := max(4, m.height-lipgloss.Height(header)-lipgloss.Height(feedback)-lipgloss.Height(footer))
	m.resizeReviewForHeight(bodyHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.filesPanel(bodyHeight), " ", m.reviewPanel(bodyHeight))
	if m.width < 76 || (m.mode == "preview" && m.readerFocus) {
		body = m.reviewPanel(bodyHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, feedback, footer)
}

// Layout Sizing

// resizeReview recalculates viewport dimensions after terminal size changes.
func (m *Model) resizeReview() {
	m.resizeReviewForHeight(max(4, m.height-lipgloss.Height(m.header())-2))
}

// resizeReviewForHeight sizes the viewport within the current body height.
func (m *Model) resizeReviewForHeight(height int) {
	reviewWidth := max(20, m.width-m.filesWidth()-5)
	if m.width < 76 || (m.mode == "preview" && m.readerFocus) {
		reviewWidth = max(20, m.width-4)
	}
	m.review.Width = reviewWidth - 4
	m.review.Height = max(1, height-3)
}

// filesWidth returns the responsive width for the changed-files panel.
func (m Model) filesWidth() int {
	return clamp(m.width/4, 24, 38)
}

// panelWidth returns the width used by full-width header panels.
func (m Model) panelWidth() int {
	return max(20, m.width-4)
}

// Header Panels

// header renders status, Git output, and any active input prompt.
func (m Model) header() string {
	parts := []string{m.topBar(), m.outputBar()}
	if m.mode == "commit" {
		parts = append(parts, commitBoxStyle(m.width).Render(titleStyle.Render("Commit message")+"\n"+m.commit.View()+"\n"+mutedStyle.Render(m.commitPromptHelp())))
	}
	if m.mode == "new-branch" {
		parts = append(parts, commitBoxStyle(m.width).Render(titleStyle.Render("Create branch")+"\n"+m.branchInput.View()+"\n"+mutedStyle.Render("enter: create  esc: cancel")))
	}
	if m.mode == "delete-branch" {
		parts = append(parts, commitBoxStyle(m.width).Render(titleStyle.Render("Delete branch")+"\n"+m.notice+"\n"+mutedStyle.Render("l: local  r: local + remote  esc: cancel")))
	}
	if m.mode == "discard-file" {
		parts = append(parts, commitBoxStyle(m.width).Render(titleStyle.Render("Discard file changes")+"\n"+m.notice+"\n"+mutedStyle.Render("y: discard  n/esc: cancel")))
	}
	if m.mode == "force-push" {
		parts = append(parts, commitBoxStyle(m.width).Render(titleStyle.Render("Force push")+"\n"+m.notice+"\n"+mutedStyle.Render("y: force push (--force-with-lease)  n/esc: cancel")))
	}
	return strings.Join(parts, "\n")
}

// outputBar renders compact or expanded Git command output.
func (m Model) outputBar() string {
	if m.fileFilterActive {
		return m.fileFilterOutputBar()
	}
	if m.mode == "search" {
		return m.searchOutputBar()
	}
	if query := strings.TrimSpace(m.fileFilter.Value()); query != "" {
		return m.fileFilterSummaryBar()
	}

	if m.loading {
		output := strings.TrimSpace(m.gitOutput)
		if output == "" {
			output = "working..."
		}
		return panelStyle.Width(m.panelWidth()).Render(m.outputBarPrefix() + warningStyle.Copy().Bold(true).Render("● RUNNING") + "  " + m.spinner.View() + " " + mutedStyle.Render(output))
	}

	output := strings.TrimSpace(m.gitOutput)
	if output == "" {
		output = "Ready — no command run yet"
	}
	style := mutedStyle
	indicator := successStyle.Copy().Bold(true).Render("✓ GIT")
	if m.err != nil {
		style = errorStyle
		indicator = errorStyle.Copy().Bold(true).Render("! GIT")
	}

	contentWidth := max(20, m.panelWidth()-6)
	compact := oneLine(output)
	if !m.outputExpanded {
		return panelStyle.Width(m.panelWidth()).Render(m.outputBarPrefix() + indicator + "  " + style.Render(trimMiddle(compact, contentWidth)))
	}

	lines, truncated := visibleOutputLines(output, contentWidth, m.outputMaxLines())
	var b strings.Builder
	b.WriteString(m.outputBarPrefix())
	b.WriteString(indicator)
	if truncated > 0 {
		fmt.Fprintf(&b, " %s", mutedStyle.Render(fmt.Sprintf("(%d earlier lines hidden)", truncated)))
	}
	b.WriteString("\n")
	b.WriteString(style.Render(strings.Join(lines, "\n")))
	return panelStyle.Width(m.panelWidth()).Render(b.String())
}

func (m Model) outputBarPrefix() string {
	path := m.selectedPath()
	if path == "" || m.mode != "review" {
		return ""
	}
	return keyStyle.Render("FILE") + " " + mutedStyle.Render(path) + "  ·  "
}

func (m Model) searchOutputBar() string {
	query := strings.TrimSpace(m.searchInput.Value())
	summary := "type word or wildcard"
	if query != "" {
		summary = m.searchSummary(query)
	}
	help := "enter/esc close  ↑ prev  ↓ next"
	content := keyStyle.Render("find ") + m.searchInput.View() + "  " + mutedStyle.Render(summary+"  "+help)
	return panelStyle.Width(m.panelWidth()).Render(content)
}

func (m Model) fileFilterOutputBar() string {
	return m.fileFilterSummaryBar()
}

func (m Model) fileFilterSummaryBar() string {
	query := strings.TrimSpace(m.fileFilter.Value())
	total := len(m.files)
	matches := len(m.filteredFiles())
	summary := fmt.Sprintf("%d/%d files", matches, total)
	if query == "" {
		summary = fmt.Sprintf("%d files", total)
	}
	help := "enter keep  esc clear  type to filter"
	content := keyStyle.Render("files ") + m.fileFilter.View() + "  " + mutedStyle.Render(summary+"  "+help)
	return panelStyle.Width(m.panelWidth()).Render(content)
}

// outputMaxLines caps expanded Git output so the main body stays usable.
func (m Model) outputMaxLines() int {
	if m.height <= 0 {
		return 8
	}
	if m.outputExpanded {
		return clamp(m.height-10, 6, 30)
	}
	return clamp(m.height/3, 3, 12)
}

// topBar renders repository metadata from git.RepoInfo.
func (m Model) topBar() string {
	upstream := m.info.Upstream
	if upstream == "" {
		upstream = "no upstream"
	}
	lastPull := "never"
	if m.info.LastPullSet {
		lastPull = m.info.LastPull.Format("Jan 02 15:04")
	}
	repo := m.info.Repo
	if repo == "" {
		repo = "no repo"
	}

	// On normal terminals the repository identity and its health get separate
	// lines. This makes the status readable at a glance instead of presenting
	// every value with the same visual weight.
	section := titleStyle.Render("GITM8") + "  " + mutedStyle.Render("/") + "  " + keyStyle.Render(strings.ToUpper(viewerTitle(m.mode)))
	identity := activeStyle.Copy().Bold(true).Render(repo) + "  " + mutedStyle.Render("on "+m.info.Branch)
	if m.width < 72 {
		available := max(16, m.panelWidth()-4)
		if lipgloss.Width(section) > available {
			modeWidth := max(4, available-len("GITM8  "))
			section = titleStyle.Render("GITM8") + "  " + keyStyle.Render(trimMiddle(strings.ToUpper(viewerTitle(m.mode)), modeWidth))
		}
		repoWidth := available - lipgloss.Width(section) - 2
		if repoWidth >= 6 {
			section += "  " + activeStyle.Copy().Bold(true).Render(trimMiddle(repo, repoWidth))
		}
		return panelStyle.Width(m.panelWidth()).Render(section)
	}

	innerWidth := max(20, m.panelWidth()-4)
	identityBudget := max(12, innerWidth-lipgloss.Width(section)-2)
	branchBudget := max(5, identityBudget/2)
	repoBudget := max(5, identityBudget-branchBudget-4)
	identity = activeStyle.Copy().Bold(true).Render(trimMiddle(repo, repoBudget)) + "  " + mutedStyle.Render("on "+trimMiddle(m.info.Branch, branchBudget))
	firstLine := spreadLine(section, identity, innerWidth)
	changeCount := m.info.Staged + m.info.Unstaged
	state := successStyle.Copy().Bold(true).Render("● CLEAN")
	changeDetail := mutedStyle.Render("working tree clear")
	if changeCount > 0 {
		state = warningStyle.Copy().Bold(true).Render(fmt.Sprintf("● %d CHANGES", changeCount))
		changeDetail = mutedStyle.Render(fmt.Sprintf("%d staged · %d working", m.info.Staged, m.info.Unstaged))
	}
	sync := successStyle.Copy().Bold(true).Render("↕ SYNCED")
	if m.info.Ahead > 0 || m.info.Behind > 0 {
		sync = warningStyle.Copy().Bold(true).Render(fmt.Sprintf("↑ %d  ↓ %d", m.info.Ahead, m.info.Behind))
	}
	status := strings.Join([]string{state, changeDetail, sync}, "  ")
	contextText := upstream + "  ·  pulled " + lastPull
	if m.width >= 118 && strings.TrimSpace(m.info.User) != "" {
		contextText += "  ·  " + m.info.User
	}
	contextBudget := innerWidth - lipgloss.Width(status) - 2
	context := ""
	if contextBudget >= 8 {
		context = mutedStyle.Render(trimMiddle(contextText, contextBudget))
	}
	secondLine := status
	if context != "" {
		secondLine = spreadLine(status, context, innerWidth)
	}
	return panelStyle.Width(m.panelWidth()).Render(firstLine + "\n" + secondLine)
}

// spreadLine keeps high-value context at opposite edges of a panel while
// gracefully collapsing to a normal inline layout in tighter terminals.
func spreadLine(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left + "  " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) feedbackBar() string {
	if strings.TrimSpace(m.toast) == "" {
		return ""
	}
	style, badge := activeStyle, "✓"
	if m.loading {
		badge = m.spinner.View()
	}
	if m.toastError {
		style, badge = errorStyle, "!"
	}
	return panelStyle.Width(m.panelWidth()).Render(style.Copy().Bold(true).Render(badge+" ") + trimMiddle(strings.TrimSpace(m.toast), max(20, m.width-10)))
}

// Body Panels

// filesPanel renders the changed-files list and keeps the cursor visible.
func (m Model) filesPanel(height int) string {
	return m.filesPanelWithRows(height, m.filteredFiles())
}

func (m Model) filesPanelWithRows(height int, files []git.FileStatus) string {
	width := m.filesWidth()
	panelHeight := max(1, height-2)
	visibleRows := max(1, panelHeight-4)

	headerWidth := max(8, width-3)
	header := spreadLine(titleStyle.Render("CHANGED FILES"), keyStyle.Render(fmt.Sprintf("%d", len(files))), headerWidth)
	legend := successStyle.Render("S") + mutedStyle.Render(" staged  ") + warningStyle.Render("U") + mutedStyle.Render(" working")
	lines := []string{header, legend, ""}
	end := min(len(files), m.fileOffset+visibleRows)
	for i := m.fileOffset; i < end; i++ {
		file := files[i]
		rowWidth := max(8, width-2)
		name := fileListLabel(file, max(4, rowWidth-7))
		if i == m.fileCursor {
			lines = append(lines, selectedStyle.Width(rowWidth).Render("▸ "+statusBadgeLabel(file)+" "+name))
		} else {
			lines = append(lines, "  "+statusBadge(file)+" "+name)
		}
	}
	if len(m.files) == 0 {
		lines = append(lines, successStyle.Render("✓ Working tree clean"), mutedStyle.Render("Nothing needs your attention."))
	} else if len(files) == 0 {
		lines = append(lines, mutedStyle.Render("No files match this filter."))
	}
	if len(files) > visibleRows {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d-%d of %d", m.fileOffset+1, end, len(files))))
	}

	return panelStyle.Width(width).Height(panelHeight).Render(strings.Join(lines, "\n"))
}

// fileListLabel adds just enough parent-path context to distinguish files with
// the same basename without turning the sidebar into a wall of paths.
func fileListLabel(file git.FileStatus, width int) string {
	name := fileListName(file)
	if file.OldPath != "" || width < 18 {
		return trimMiddle(name, width)
	}
	dir := filepath.Dir(strings.TrimSuffix(file.Path, "/"))
	if dir == "." || dir == "" {
		return trimMiddle(name, width)
	}
	location := dir + "/"
	if lipgloss.Width(name)+lipgloss.Width(location)+2 > width {
		location = trimMiddle(location, max(5, width/2))
	}
	name = trimMiddle(name, max(4, width-lipgloss.Width(location)-2))
	return name + "  " + mutedStyle.Render(location)
}

// reviewPanel renders the active viewer title and viewport content.
func (m Model) reviewPanel(height int) string {
	reviewWidth := max(20, m.width-m.filesWidth()-5)
	if m.width < 76 || (m.mode == "preview" && m.readerFocus) {
		reviewWidth = max(20, m.width-4)
	}
	reviewHeight := max(1, height-2)
	if m.mode == "preview" {
		meta := m.fileReaderHeader(reviewWidth)
		readerHeight := max(1, reviewHeight-lipgloss.Height(meta)-1)
		m.review.Width = max(12, reviewWidth-8)
		m.review.Height = readerHeight
		reader := lipgloss.JoinHorizontal(lipgloss.Top, m.review.View(), " ", m.readerRail(readerHeight))
		return activePanelStyle.Width(reviewWidth).Height(reviewHeight).Render(meta + "\n" + reader)
	}
	title := panelHeading(strings.ToUpper(viewerTitle(m.mode)), m.viewerContext(), max(16, reviewWidth-4))
	return activePanelStyle.Width(reviewWidth).Height(reviewHeight).Render(title + "\n" + m.review.View())
}

func panelHeading(title, context string, width int) string {
	left := keyStyle.Render("◆") + "  " + titleStyle.Render(title)
	if strings.TrimSpace(context) == "" {
		return left
	}
	contextWidth := width - lipgloss.Width(left) - 2
	if contextWidth < 6 {
		return left
	}
	return spreadLine(left, mutedStyle.Render(trimMiddle(context, contextWidth)), width)
}

func (m Model) viewerContext() string {
	switch m.mode {
	case "review":
		if len(m.files) == 1 {
			return "1 changed file"
		}
		return fmt.Sprintf("%d changed files", len(m.files))
	case "branches", "rebase":
		return fmt.Sprintf("%d branches", len(m.branches))
	case "conflicts":
		return fmt.Sprintf("%d unresolved", len(m.conflicts))
	case "stashes":
		return fmt.Sprintf("%d saved", len(m.stashes))
	case "squash":
		return fmt.Sprintf("%d commits", len(m.commits))
	default:
		return ""
	}
}

func (m Model) fileReaderHeader(width int) string {
	path := m.target
	if path == "" {
		path = "file"
	}
	name := filepath.Base(path)
	directory := filepath.Dir(path)
	headingWidth := max(12, width-4)
	heading := keyStyle.Render("◇ ") + titleStyle.Render(ansi.Truncate(name, headingWidth-2, "…"))
	if directory != "." {
		remaining := headingWidth - lipgloss.Width(heading) - 3
		if remaining >= 10 {
			heading = spreadLine(heading, mutedStyle.Render(ansi.Truncate(directory+"/", remaining, "…")), headingWidth)
		}
	}
	lines := 0
	if m.viewerContent != "" {
		lines = strings.Count(strings.TrimSuffix(m.viewerContent, "\n"), "\n") + 1
	}
	status := "tracked"
	if file, ok := m.selectedFile(); ok {
		status = strings.TrimSpace(file.Label())
		if status == "" {
			status = "tracked"
		}
	}
	language := fileLanguage(path)
	mode := "SPLIT"
	if m.readerFocus {
		mode = "FOCUS"
	}
	metadataItems := []string{keyStyle.Render(strings.ToUpper(language)), mutedStyle.Render(fmt.Sprintf("%d lines", lines)), mutedStyle.Render(humanBytes(int64(len(m.viewerContent)))), activeStyle.Render(status), keyStyle.Render(mode)}
	if width < 66 {
		metadataItems = []string{keyStyle.Render(strings.ToUpper(language)), mutedStyle.Render(fmt.Sprintf("%dL", lines)), keyStyle.Render(mode)}
	}
	metadata := strings.Join(metadataItems, "  •  ")
	return heading + "\n" + metadata
}

func (m Model) focusedReaderView() string {
	repo := m.info.Repo
	if repo == "" {
		repo = "repository"
	}
	readerBrand := titleStyle.Render("GITM8") + "  " + keyStyle.Render("// READER")
	readerWidth := max(20, m.width-2)
	readerContextWidth := readerWidth - lipgloss.Width(readerBrand) - 2
	top := " " + readerBrand
	if readerContextWidth >= 8 {
		readerContext := mutedStyle.Render(trimMiddle(repo+" on "+m.info.Branch, readerContextWidth))
		top = " " + spreadLine(readerBrand, readerContext, readerWidth)
	}
	footer := mutedStyle.Width(max(20, m.width)).Render(strings.Join([]string{keyHint("F", "split view"), keyHint("/", "find"), keyHint("j/k", "read"), keyHint("enter", "edit"), keyHint("q", "quit")}, "  "))
	bodyHeight := max(8, m.height-lipgloss.Height(top)-lipgloss.Height(footer)-1)
	body := m.reviewPanel(bodyHeight)
	return lipgloss.JoinVertical(lipgloss.Left, top, body, footer)
}

func fileLanguage(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	names := map[string]string{"go": "Go", "js": "JavaScript", "ts": "TypeScript", "tsx": "TypeScript React", "jsx": "React", "md": "Markdown", "rs": "Rust", "py": "Python", "rb": "Ruby", "sh": "Shell", "yaml": "YAML", "yml": "YAML", "json": "JSON", "toml": "TOML", "css": "CSS", "html": "HTML"}
	if name := names[ext]; name != "" {
		return name
	}
	if ext == "" {
		return "Text"
	}
	return strings.ToUpper(ext)
}

func (m Model) readerRail(height int) string {
	if height <= 0 {
		return ""
	}
	position := clamp(int(m.review.ScrollPercent()*float64(height-1)), 0, height-1)
	rows := make([]string, height)
	for i := range rows {
		rows[i] = mutedStyle.Render("│")
	}
	rows[position] = keyStyle.Render("┃")
	return strings.Join(rows, "\n")
}

// Footer

// footer renders the dashboard key reference unless the user hides it.
func (m Model) footer() string {
	if m.footerHidden {
		return ""
	}
	rows := m.footerRows()
	if len(rows) == 0 {
		return ""
	}
	return mutedStyle.Width(max(20, m.width)).Render(strings.Join(rows, "\n"))
}

// shortcutVisible controls presentation only; key handling remains unchanged.
func (m Model) shortcutVisible(id string) bool {
	if len(m.config.KeyReference) == 0 {
		for _, defaultID := range m.defaultMainFooterIDs() {
			if defaultID == id {
				return true
			}
		}
		return false
	}
	for _, selected := range m.config.KeyReference {
		if selected == id {
			return true
		}
	}
	return false
}

func (m Model) defaultMainFooterIDs() []string {
	ids := []string{"help", "move-files", "find-file", "quit"}
	if m.width >= 96 {
		ids = []string{"help", "move-files", "edit-file", "focus-reader", "find-file", "commit", "push-pull", "quit"}
	}
	return ids
}

func mainFooterLabel(id string) string {
	labels := map[string]string{
		"move-files": "files", "edit-file": "edit", "review-repository": "review",
		"toggle-diff": "diff", "find-file": "find", "discard-file": "discard",
		"stage-files": "stage", "unstage-files": "unstage", "stash-file": "stash",
		"commit": "commit", "fetch": "fetch", "push-pull": "push/pull",
		"squash": "squash", "conflicts": "conflicts", "stashes": "stashes",
		"branches": "branches", "switch-with-changes": "switch+changes", "logs": "logs",
		"identity": "identity", "pull-request": "PR", "releases": "releases",
		"command-palette": "commands", "themes": "themes", "focus-reader": "focus",
		"rebase": "rebase", "help": "all keys", "output": "output",
		"toggle-keybar": "key bar", "yazi": "yazi", "scroll-lines": "scroll",
		"scroll-page": "page", "jump-viewer": "top/bottom", "quit": "quit",
	}
	return labels[id]
}

func (m Model) mainFooterRows() []string {
	var items []string
	for _, key := range m.helpKeys() {
		if m.shortcutVisible(key.id) {
			label := mainFooterLabel(key.id)
			if key.id == "find-file" {
				label = "filter files"
				if m.mode == "preview" {
					label = "find in file"
				}
			}
			items = append(items, keyHint(key.key, label))
		}
	}
	var rows []string
	line := ""
	for _, item := range items {
		candidate := item
		if line != "" {
			candidate = line + "  " + item
		}
		if line != "" && lipgloss.Width(candidate) > max(20, m.width) {
			rows = append(rows, line)
			line = item
			continue
		}
		line = candidate
	}
	if line != "" {
		rows = append(rows, line)
	}
	return rows
}

func keyHint(key, label string) string {
	return keycapStyle.Render(key) + " " + mutedStyle.Render(label)
}

func (m Model) footerRows() []string {
	switch m.mode {
	case "branches":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[↑/↓]") + " choose branch",
				keyStyle.Render("[enter]") + " switch",
				keyStyle.Render("[W]") + " switch with changes",
				keyStyle.Render("[n]") + " create",
			}, "  "),
			strings.Join([]string{
				keyStyle.Render("[D]") + " delete",
				keyStyle.Render("[r]") + " refresh",
				keyStyle.Render("[esc]") + " return",
			}, "  "),
		}
	case "rebase":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[↑/↓]") + " choose target",
				keyStyle.Render("[enter]") + " rebase",
				keyStyle.Render("[c]") + " continue",
				keyStyle.Render("[a]") + " abort",
			}, "  "),
			strings.Join([]string{
				keyStyle.Render("[s]") + " skip",
				keyStyle.Render("[r]") + " refresh",
				keyStyle.Render("[esc]") + " cancel",
			}, "  "),
		}
	case "conflicts":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[↑/↓]") + " choose file",
				keyStyle.Render("[enter]") + " open editor",
				keyStyle.Render("[m]") + " mark resolved",
				keyStyle.Render("[c]") + " continue",
			}, "  "),
			strings.Join([]string{
				keyStyle.Render("[a]") + " abort",
				keyStyle.Render("[s]") + " skip",
				keyStyle.Render("[r]") + " refresh",
				keyStyle.Render("[esc]") + " return",
			}, "  "),
		}
	case "stashes":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[↑/↓]") + " choose stash",
				keyStyle.Render("[n]") + " stash new",
				keyStyle.Render("[a]") + " apply",
				keyStyle.Render("[p]") + " pop",
			}, "  "),
			strings.Join([]string{
				keyStyle.Render("[D]") + " drop",
				keyStyle.Render("[r]") + " refresh",
				keyStyle.Render("[esc]") + " return",
			}, "  "),
		}
	case "releases":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[↑/↓]") + " choose",
				keyStyle.Render("[n]") + " new release",
				keyStyle.Render("[r]") + " refresh",
				keyStyle.Render("[esc]") + " return",
			}, "  "),
		}
	case "squash":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[↑/↓]") + " choose commit",
				keyStyle.Render("[S]") + " squash",
				keyStyle.Render("[K]") + " keep",
				keyStyle.Render("[B]") + " base",
			}, "  "),
			strings.Join([]string{
				keyStyle.Render("[a]") + " mark all",
				keyStyle.Render("[enter]") + " apply",
				keyStyle.Render("[r]") + " refresh",
				keyStyle.Render("[esc]") + " cancel",
			}, "  "),
		}
	case "profiles":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[↑/↓]") + " choose profile",
				keyStyle.Render("[enter]") + " apply",
				keyStyle.Render("[esc]") + " return",
			}, "  "),
		}
	case "pull-request":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[g]") + " generate",
				keyStyle.Render("[m]") + " write it yourself",
				keyStyle.Render("[esc]") + " return",
			}, "  "),
		}
	case "commit":
		lines := []string{
			strings.Join([]string{
				keyStyle.Render("[enter]") + " commit",
				keyStyle.Render("[esc]") + " cancel",
			}, "  "),
		}
		if m.config.AIAvailable {
			lines = append([]string{strings.Join([]string{keyStyle.Render("[ctrl+g]") + " generate subject"}, "  ")}, lines...)
		}
		return lines
	case "new-branch":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[enter]") + " create branch",
				keyStyle.Render("[esc]") + " cancel",
			}, "  "),
		}
	case "delete-branch":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[l]") + " local only",
				keyStyle.Render("[r]") + " local + remote",
				keyStyle.Render("[esc]") + " cancel",
			}, "  "),
		}
	case "discard-file":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[y]") + " discard changes",
				keyStyle.Render("[esc]") + " cancel",
			}, "  "),
		}
	case "force-push":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[y]") + " force push",
				keyStyle.Render("[esc]") + " cancel",
			}, "  "),
		}
	case "search":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[enter]") + " jump to match",
				keyStyle.Render("[↑/↓]") + " match navigation",
				keyStyle.Render("[esc]") + " close",
			}, "  "),
		}
	case "help":
		return []string{
			strings.Join([]string{
				keyStyle.Render("[j/k]") + " scroll",
				keyStyle.Render("[esc]") + " return",
			}, "  "),
		}
	default:
		return m.mainFooterRows()
	}
}

// Small View Helpers

// statusBadge turns Git status values into short staged/unstaged labels.
func statusBadge(file git.FileStatus) string {
	label := statusBadgeLabel(file)
	switch {
	case file.Renamed():
		return keyStyle.Render(label)
	case file.Deleted():
		return errorStyle.Render(label)
	case file.Index == '?':
		return keyStyle.Render(label)
	case file.Staged() && file.Unstaged():
		return warningStyle.Render(label)
	case file.Staged():
		return successStyle.Render(label)
	case file.Unstaged():
		return warningStyle.Render(label)
	default:
		return mutedStyle.Render(label)
	}
}

func statusBadgeLabel(file git.FileStatus) string {
	switch {
	case file.Renamed():
		return "REN"
	case file.Deleted():
		return "DEL"
	case file.Staged() && file.Unstaged():
		return "S/U"
	case file.Staged():
		return "S  "
	case file.Unstaged():
		return " U "
	default:
		label := file.Label()
		if len(label) < 3 {
			label += strings.Repeat(" ", 3-len(label))
		}
		return label
	}
}

// viewerTitle turns internal screen names into panel titles.
func viewerTitle(mode string) string {
	switch mode {
	case "preview":
		return "File Preview"
	case "branches":
		return "Switch Branch"
	case "rebase":
		return "Rebase"
	case "squash":
		return "Squash Commits"
	case "logs":
		return "Commit Logs"
	case "profiles":
		return "Switch Identity"
	case "pull-request":
		return "Pull Request"
	case "help":
		return "Help"
	case "conflicts":
		return "Conflicts"
	case "stashes":
		return "Stashes"
	case "releases":
		return "GitHub Releases"
	case "release-create":
		return "Create Release"
	case "new-branch":
		return "Create Branch"
	case "delete-branch":
		return "Delete Branch"
	case "discard-file":
		return "Discard File"
	case "force-push":
		return "Force Push"
	case "commit":
		return "Commit"
	case "search":
		return "Find In File"
	default:
		return "Code Review"
	}
}

// commitBoxStyle returns the full-width style used by transient input prompts.
func commitBoxStyle(width int) lipgloss.Style {
	return panelStyle.Width(max(20, width-4))
}

// Picker Views

// branchesView renders the current model's branch picker.
func (m Model) branchesView() string {
	height := max(8, m.height-8)
	visibleRows := max(1, height-4)
	return branchesView(m.branches, m.branchCursor, m.branchOffset, visibleRows)
}

// rebaseView renders the branch picker with rebase-specific controls.
func (m Model) rebaseView() string {
	if len(m.branches) == 0 {
		return "No branches found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Rebase %s onto a branch. ↑/↓ choose, enter rebase, c continue, a abort, s skip, esc cancel.\n\n", m.info.Branch)
	visibleRows := max(1, max(8, m.height-8)-4)
	end := min(len(m.branches), m.branchOffset+visibleRows)
	for i := m.branchOffset; i < end; i++ {
		pointer := "  "
		if i == m.branchCursor {
			pointer = "> "
		}
		marker := ""
		if m.branches[i] == m.info.Branch {
			marker = "  (current)"
		}
		fmt.Fprintf(&b, "%s%s%s\n", pointer, m.branches[i], marker)
	}
	if len(m.branches) > visibleRows {
		fmt.Fprintf(&b, "\n%d-%d of %d\n", m.branchOffset+1, end, len(m.branches))
	}
	return b.String()
}

// squashView renders the in-TUI squash picker. Commits are listed newest first;
// S marks a commit to squash, K keeps it, and B chooses the base commit that
// receives the squashed commits.
func (m Model) squashView() string {
	if len(m.commits) == 0 {
		return "No commits to squash on this branch.\n"
	}

	folds := m.squashFoldCount()
	baseSelected := m.squashBaseSelected()

	var b strings.Builder
	fmt.Fprintf(&b, "Squash commits on %s since %s, without leaving gitm8.\n", m.info.Branch, m.config.DefaultBranch)
	b.WriteString("↑/↓ move, S squash, K keep, B base, a mark all squash, enter apply, r refresh, esc cancel.\n")
	b.WriteString(mutedStyle.Render("Mark commits first, then choose exactly one base commit with B. The base message is used for the combined commit.") + "\n\n")

	if folds > 0 && baseSelected {
		fmt.Fprintf(&b, "%s\n\n", keyStyle.Render(fmt.Sprintf("Squashing %d commit(s) into %s, leaving %d.", folds, m.commits[m.squashBase].Hash, len(m.commits)-folds)))
	} else if folds > 0 {
		fmt.Fprintf(&b, "%s\n\n", mutedStyle.Render(fmt.Sprintf("%d commit(s) marked to squash. Choose a base with B.", folds)))
	} else {
		fmt.Fprintf(&b, "%s\n\n", mutedStyle.Render("Nothing to squash yet - mark one or more commits with S."))
	}

	visibleRows := max(1, max(8, m.height-8)-7)
	end := min(len(m.commits), m.squashOffset+visibleRows)
	for i := m.squashOffset; i < end; i++ {
		pointer := "  "
		if i == m.squashCursor {
			pointer = keyStyle.Render("> ")
		}
		commit := m.commits[i]
		var tag string
		switch {
		case baseSelected && i == m.squashBase:
			tag = keyStyle.Render("base  ")
		case m.squashMark[i]:
			tag = activeStyle.Render("squash")
		default:
			tag = mutedStyle.Render("keep  ")
		}
		fmt.Fprintf(&b, "%s%s  %s  %s\n", pointer, tag, mutedStyle.Render(commit.Hash), commit.Subject)
	}
	if len(m.commits) > visibleRows {
		fmt.Fprintf(&b, "\n%d-%d of %d\n", m.squashOffset+1, end, len(m.commits))
	}
	return b.String()
}

// squashUnavailableView explains why squashing is not possible right now.
func (m Model) squashUnavailableView(err error) string {
	reason := "squashing is not available right now"
	if err != nil {
		reason = err.Error()
	}
	return "Cannot squash: " + reason + "\n\nFix the issue above and press z to try again.\n"
}

// branchesView renders branch rows from plain inputs so it is easy to test.
func branchesView(branches []string, cursor int, offset int, visibleRows int) string {
	if len(branches) == 0 {
		return "No branches found.\n"
	}
	var b strings.Builder
	b.WriteString("Select a branch with ↑/↓, enter to switch, W to switch with changes, n to create, D to delete, or esc to return.\n\n")
	end := min(len(branches), offset+visibleRows)
	for i := offset; i < end; i++ {
		pointer := "  "
		if i == cursor {
			pointer = "> "
		}
		fmt.Fprintf(&b, "%s%s\n", pointer, branches[i])
	}
	if len(branches) > visibleRows {
		fmt.Fprintf(&b, "\n%d-%d of %d\n", offset+1, end, len(branches))
	}
	return b.String()
}

// profilesView renders configured Git identity profiles and marks the current one.
func (m Model) profilesView() string {
	profiles := m.config.Profiles
	if len(profiles) == 0 {
		return "No profiles configured.\n\nAdd identities to ~/.gitm8/profiles, e.g.\n\n  Work = Ada Lovelace <ada@work.example>\n  Personal = Ada <ada@personal.example>\n"
	}
	var b strings.Builder
	b.WriteString("Select an identity with ↑/↓, enter to apply to this repo, or esc to return.\n\n")
	visibleRows := max(1, max(8, m.height-8)-4)
	end := min(len(profiles), m.profileOffset+visibleRows)
	for i := m.profileOffset; i < end; i++ {
		pointer := "  "
		if i == m.profileCursor {
			pointer = "> "
		}
		profile := profiles[i]
		current := ""
		if profile.Name == m.info.User || profile.Email == m.info.User {
			current = "  (current)"
		}
		fmt.Fprintf(&b, "%s%s — %s <%s>%s\n", pointer, profile.Label, profile.Name, profile.Email, current)
	}
	if len(profiles) > visibleRows {
		fmt.Fprintf(&b, "\n%d-%d of %d\n", m.profileOffset+1, end, len(profiles))
	}
	return b.String()
}

// conflictsView renders conflict rows, marker lines, and the selected file preview.
func (m Model) conflictsView(markerReport string, preview string) string {
	if len(m.conflicts) == 0 {
		return "No conflict files found.\n\nUse normal Git commands or pull/rebase again when ready.\n"
	}

	var b strings.Builder
	b.WriteString("Resolve conflicts, then press m to mark resolved. enter opens the file in your editor. c continue, a abort, s skip, r refresh, esc return.\n\n")
	visibleRows := max(1, max(8, m.height-8)-4)
	end := min(len(m.conflicts), m.conflictOffset+visibleRows)
	for i := m.conflictOffset; i < end; i++ {
		pointer := "  "
		if i == m.conflictCursor {
			pointer = "> "
		}
		fmt.Fprintf(&b, "%s%s\n", pointer, m.conflicts[i])
	}
	if len(m.conflicts) > visibleRows {
		fmt.Fprintf(&b, "\n%d-%d of %d\n", m.conflictOffset+1, end, len(m.conflicts))
	}
	b.WriteString("\n")
	if strings.TrimSpace(markerReport) != "" {
		b.WriteString(strings.TrimSpace(markerReport))
		b.WriteString("\n\n")
	}
	b.WriteString(strings.TrimSpace(preview))
	b.WriteString("\n")
	return b.String()
}

// stashesView renders stash rows above the selected stash diff.
func (m Model) stashesView(diff string) string {
	if len(m.stashes) == 0 {
		return "No stashes found.\n\nPress n to stash current changes, or esc to return.\n"
	}

	var b strings.Builder
	b.WriteString("Stashes: n new, a apply, p pop, D drop, r refresh, esc return. j/k scroll the diff.\n\n")
	visibleRows := max(1, max(8, m.height-8)-4)
	end := min(len(m.stashes), m.stashOffset+visibleRows)
	for i := m.stashOffset; i < end; i++ {
		pointer := "  "
		if i == m.stashCursor {
			pointer = "> "
		}
		stash := m.stashes[i]
		fmt.Fprintf(&b, "%s%s  %s\n", pointer, stash.Ref, stash.Subject)
	}
	if len(m.stashes) > visibleRows {
		fmt.Fprintf(&b, "\n%d-%d of %d\n", m.stashOffset+1, end, len(m.stashes))
	}
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(diff))
	b.WriteString("\n")
	return b.String()
}

// pullRequestView renders the PR creation choice.
func (m Model) pullRequestView() string {
	provider := m.config.AIProvider
	if provider == "" {
		provider = "codex"
	}
	if !m.config.AIAvailable {
		return fmt.Sprintf("Create a pull request for the current branch.\n\n  m  write it yourself with gh pr create\n\n  AI generation unavailable: %s\n\n  esc  cancel\n", strings.TrimPrefix(m.aiUnavailableNotice(), "AI generation unavailable: "))
	}
	return fmt.Sprintf("Create a pull request for the current branch.\n\n  g  generate title and description with %s\n  m  write it yourself with gh pr create\n\n  esc  cancel\n", provider)
}

func (m Model) commitPromptHelp() string {
	if !m.config.AIAvailable {
		return "enter: commit  esc: cancel"
	}
	return "ctrl+g: generate  enter: commit  esc: cancel"
}

// Help View

type helpKey struct {
	id          string
	key         string
	description string
}

type helpListItem struct {
	key      helpKey
	selected bool
}

func (i helpListItem) FilterValue() string {
	return i.key.id + " " + i.key.key + " " + i.key.description
}

func (i helpListItem) Title() string {
	mark := "[ ]"
	if i.selected {
		mark = "[x]"
	}
	return fmt.Sprintf("%s  %-16s %s", mark, i.key.key, i.key.description)
}

func (i helpListItem) Description() string {
	return ""
}

func (m Model) helpKeys() []helpKey {
	commitHelp := "commit staged changes"
	if m.config.AIAvailable {
		commitHelp = "commit staged changes (ctrl+g generates a message in commit mode)"
	}
	pushHelp := "pull (--ff-only) / push (offers force-with-lease if rejected)"
	if m.config.MattMode {
		pushHelp = "pull (--ff-only) / push (matt_mode: always --force, no safety net)"
	}
	return []helpKey{
		{"move-files", "↑/↓", "move file selection (previews it)"},
		{"edit-file", "enter", "open the viewed file in GITM8_EDITOR"},
		{"review-repository", "0", "back to the repo-wide code review"},
		{"toggle-diff", "d", "toggle the selected file between diff and contents"},
		{"find-file", "/", "search and highlight inside the viewed file contents"},
		{"discard-file", "x", "discard all changes to selected file"},
		{"stage-files", "s / S", "stage selected file / stage all"},
		{"unstage-files", "u / U", "unstage selected file / unstage all"},
		{"stash-file", "n", "stash selected file"},
		{"commit", "c", commitHelp},
		{"fetch", "f", "fetch (--all --prune)"},
		{"push-pull", "p / P", pushHelp},
		{"squash", "z", "mark commits, choose a base, squash in-TUI (no editor)"},
		{"conflicts", "C", "conflict mode for unmerged files"},
		{"stashes", "t", "stash panel"},
		{"branches", "b", "branch switcher"},
		{"switch-with-changes", "W", "in branch switcher: switch and bring current changes"},
		{"logs", "l", "view recent commit logs"},
		{"identity", "i", "identity switcher (git user profiles)"},
		{"pull-request", "r", "pull request options"},
		{"releases", "v", "list and create GitHub releases"},
		{"command-palette", ": / ctrl+k", "searchable command palette"},
		{"themes", "T", "preview and switch themes"},
		{"focus-reader", "F", "toggle focused file reader while previewing"},
		{"rebase", "R", "rebase the current branch onto another"},
		{"help", "h", "this help"},
		{"output", "o", "expand or collapse the git output box"},
		{"toggle-keybar", "tab", "hide or show the footer key bar"},
		{"yazi", "y", "open the yazi file manager"},
		{"scroll-lines", "j/k", "scroll the viewer line by line"},
		{"scroll-page", "pgdn/pgup", "scroll the viewer by a page"},
		{"jump-viewer", "g / G", "jump viewer to top / bottom"},
		{"quit", "q / ctrl+c", "quit"},
	}
}

func (m Model) filteredHelpKeys() []helpKey {
	query := strings.ToLower(strings.TrimSpace(m.helpInput.Value()))
	allKeys := m.helpKeys()
	if query == "" {
		return allKeys
	}
	type scoredHelpKey struct {
		key   helpKey
		score int
	}
	var scored []scoredHelpKey
	for _, key := range allKeys {
		haystack := strings.ToLower(key.id + " " + key.key + " " + key.description)
		if score, ok := fuzzyLineScore(haystack, query); ok {
			scored = append(scored, scoredHelpKey{key: key, score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	result := make([]helpKey, 0, len(scored))
	for _, item := range scored {
		result = append(result, item.key)
	}
	return result
}

func (m Model) helpKeySelected(id string) bool {
	if m.helpSelected == nil {
		return true
	}
	return m.helpSelected[id]
}

// helpView renders the interactive shortcut selector.
func (m Model) helpView() string {
	if m.helpListReady {
		return m.helpList.View()
	}
	keys := m.filteredHelpKeys()
	maxRows := max(4, m.height-13)
	start := clamp(m.helpCursor-maxRows+1, 0, max(0, len(keys)-maxRows))
	end := min(len(keys), start+maxRows)
	var b strings.Builder
	b.WriteString(titleStyle.Render("MAIN PANEL SHORTCUTS") + "\n")
	b.WriteString(mutedStyle.Render("Choose which labels appear in the main key bar; shortcuts stay active") + "\n")
	b.WriteString(mutedStyle.Render("Type to filter like fzf; ↑/↓ or j/k move; space toggles; enter saves") + "\n\n")
	b.WriteString(m.helpInput.View() + "\n\n")
	for i := start; i < end; i++ {
		key := keys[i]
		marker := "[ ]"
		if m.helpKeySelected(key.id) {
			marker = "[x]"
		}
		line := fmt.Sprintf("%s %-16s %s", marker, key.key, key.description)
		if i == m.helpCursor {
			line = selectedStyle.Width(max(20, m.width-8)).Render("▸ " + line)
		}
		b.WriteString(line + "\n")
	}
	if len(keys) == 0 {
		b.WriteString(mutedStyle.Render("No matching shortcuts") + "\n")
	}
	if len(keys) > maxRows {
		fmt.Fprintf(&b, "\n%s\n", mutedStyle.Render(fmt.Sprintf("%d-%d of %d", start+1, end, len(keys))))
	}
	b.WriteString("\n" + mutedStyle.Render("esc cancel  enter save  ctrl+c quit"))
	return b.String()
}

// helpScreenView gives the complete key reference the full terminal instead of
// squeezing it beside the changed-files panel.
func (m Model) helpScreenView() string {
	return m.renderHelpScreen()
}

// renderHelpScreen also lays out the list. Input handling uses the same layout
// so pagination and the highlighted item agree with what is drawn.
func (m *Model) renderHelpScreen() string {
	contentWidth := max(20, m.width-4)
	header := brandHeader(contentWidth, "ALL KEYS", "Your commands, at a glance")
	intro := titleStyle.Render("MAIN KEY BAR") + "  " + mutedStyle.Render("Choose the shortcuts shown in your footer")
	legend := keyStyle.Render("●") + mutedStyle.Render(" visible    ○ hidden    ·    All shortcuts remain active")
	hints := []string{keyHint("↑/↓", "move"), keyHint("/", "search"), keyHint("space", "toggle"), keyHint("enter", "save"), keyHint("esc", "cancel")}
	if m.helpListReady && m.helpList.FilterInput.Focused() {
		hints = []string{keyHint("type", "search"), keyHint("enter", "browse results"), keyHint("esc", "clear search")}
	}
	footer := lipgloss.NewStyle().Width(max(20, m.width)).Render(strings.Join(hints, "   "))
	innerWidth := max(16, contentWidth-2)
	detail := m.shortcutDetail(innerWidth)
	if m.toastError {
		detail = errorStyle.Render(m.toast)
	}
	summary := lipgloss.NewStyle().Width(innerWidth).Render(intro + "\n" + legend)
	bodyHeight := max(6, m.height-lipgloss.Height(header)-lipgloss.Height(footer))
	listHeight := max(3, bodyHeight-2-lipgloss.Height(summary)-lipgloss.Height(detail)-3)
	if m.helpListReady {
		m.helpList.SetSize(innerWidth, listHeight)
	}
	content := summary + "\n\n" + m.helpView() + "\n" + mutedStyle.Render(strings.Repeat("─", innerWidth)) + "\n" + detail
	body := activePanelStyle.Width(contentWidth).Height(max(1, bodyHeight-2)).Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// Splash View

// splashTickMsg tells Update that the splash animation should advance one frame.
type splashTickMsg struct{}

const (
	splashFrameCount = 14
	splashTickEvery  = 100 * time.Millisecond
	splashCardWidth  = 46
)

const splashBanner = ` ██████╗ ██╗████████╗███╗   ███╗ █████╗ 
██╔════╝ ██║╚══██╔══╝████╗ ████║██╔══██╗
██║  ███╗██║   ██║   ██╔████╔██║╚█████╔╝
██║   ██║██║   ██║   ██║╚██╔╝██║██╔══██╗
╚██████╔╝██║   ██║   ██║ ╚═╝ ██║╚█████╔╝
 ╚═════╝ ╚═╝   ╚═╝   ╚═╝     ╚═╝ ╚════╝ 

        fetch • branch • commit • push`

var splashMessages = []string{
	"rebasing expectations",
	"asking git nicely",
	"checking if main is still main",
	"finding that one missing semicolon",
	"rebaseing is based",
	"thinkpad = chadpad",
	"gits never be so delicious",
	"i use arch btw",
	"git happens",
	"i cant spell but i can code",
	"one more pr to retirement",
	"LGTM!",
	"written in go beacuse fast",
	"linux good, windows bad",
	"sometimes the most juinor of tasks requires the most senior engineers",
	"i dont need enemies; I have dependency updates",
	"git blame is workplace violence",
	"i dont write bugs - I create surprise features with timestamps",
	"i named the branch final-final-actually-final",
	"squash merge: because history is written by the winners",
	"my branch is called quick-fix, so naturally its load-bearing infrastructure now",
	"well maybe the coffees just not for you",
	"my PR is small: only changes everything",
	"my unit tests are mostly vibes",
	"this repo is maintained by caffeine",
	"version control? control is a myth",
	"written for devs by devs free forever",
	"opensauce*",
	"nothing from my end",
	"leetcode yeetcode",
	"dont deploy on friday!",
}

// updateSplash advances the startup animation and triggers the first repo load.
func (m Model) updateSplash(splashTickMsg) (tea.Model, tea.Cmd) {
	if !m.splash {
		return m, nil
	}
	m.splashFrame++
	if m.splashFrame >= splashFrameCount {
		m.splash = false
		if m.mode == "workspace" {
			return m, loadProjects(m.config.WorkspaceDir)
		}
		return m, loadDefault(m.runner)
	}
	return m, tickSplash()
}

// splashView renders the centered startup card while repository loading is delayed.
func (m Model) splashView() string {
	frame := clamp(m.splashFrame, 0, splashFrameCount-1)
	barWidth := 24
	filled := clamp((frame+1)*barWidth/splashFrameCount, 1, barWidth)
	bar := keyStyle.Render(strings.Repeat("=", filled)) + mutedStyle.Render(strings.Repeat("-", barWidth-filled))

	pulse := []string{"|", "/", "-", "\\"}[frame%4]
	barLine := lipgloss.PlaceHorizontal(splashCardWidth, lipgloss.Center, "["+bar+"]")
	lines := []string{
		"",
		titleStyle.Render(centerMultiline(splashBanner, splashCardWidth)),
		"",
		"",
		keyStyle.Render(pulse + " preparing repository view"),
		barLine,
		mutedStyle.Render(m.splashMessage),
	}
	card := panelStyle.Width(splashCardWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}

// centerMultiline centers each banner line inside the splash card.
func centerMultiline(value string, width int) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = lipgloss.PlaceHorizontal(width, lipgloss.Center, strings.TrimSpace(line))
	}
	return strings.Join(lines, "\n")
}

// randomSplashMessage chooses a startup tagline without affecting app behavior.
func randomSplashMessage() string {
	if len(splashMessages) == 0 {
		return ""
	}
	return splashMessages[rand.Intn(len(splashMessages))]
}

// tickSplash schedules the next splash animation message.
func tickSplash() tea.Cmd {
	return tea.Tick(splashTickEvery, func(time.Time) tea.Msg {
		return splashTickMsg{}
	})
}
