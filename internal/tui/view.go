package tui

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	if m.splash {
		return m.splashView()
	}
	if m.mode == "workspace" {
		return m.workspaceView()
	}

	header := m.header()
	footer := m.footer()
	bodyHeight := max(4, m.height-lipgloss.Height(header)-lipgloss.Height(footer))
	m.resizeReviewForHeight(bodyHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.filesPanel(bodyHeight), m.reviewPanel(bodyHeight))

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// Layout Sizing

// resizeReview recalculates viewport dimensions after terminal size changes.
func (m *Model) resizeReview() {
	m.resizeReviewForHeight(max(4, m.height-lipgloss.Height(m.header())-2))
}

// resizeReviewForHeight sizes the viewport within the current body height.
func (m *Model) resizeReviewForHeight(height int) {
	reviewWidth := max(20, m.width-m.filesWidth()-4)
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
		return panelStyle.Width(m.panelWidth()).Render(m.outputBarPrefix() + keyStyle.Render("git ") + m.spinner.View() + " " + mutedStyle.Render(output))
	}

	output := strings.TrimSpace(m.gitOutput)
	if output == "" {
		output = "No git output yet"
	}
	style := mutedStyle
	if m.err != nil {
		style = errorStyle
	}

	contentWidth := max(20, m.panelWidth()-6)
	compact := oneLine(output)
	if !m.outputExpanded {
		return panelStyle.Width(m.panelWidth()).Render(m.outputBarPrefix() + keyStyle.Render("git ") + style.Render(trimMiddle(compact, contentWidth)))
	}

	lines, truncated := visibleOutputLines(output, contentWidth, m.outputMaxLines())
	var b strings.Builder
	b.WriteString(m.outputBarPrefix())
	b.WriteString(keyStyle.Render("git"))
	if truncated > 0 {
		fmt.Fprintf(&b, " %s", mutedStyle.Render(fmt.Sprintf("(%d earlier lines hidden)", truncated)))
	}
	b.WriteString("\n")
	b.WriteString(style.Render(strings.Join(lines, "\n")))
	return panelStyle.Width(m.panelWidth()).Render(b.String())
}

func (m Model) outputBarPrefix() string {
	path := m.selectedPath()
	if path == "" || (m.mode != "preview" && m.mode != "review") {
		return ""
	}
	return keyStyle.Render("path ") + mutedStyle.Render(path) + "  "
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
	sync := fmt.Sprintf("+%d/-%d", m.info.Ahead, m.info.Behind)
	repo := m.info.Repo
	if repo == "" {
		repo = "no repo"
	}
	items := []string{
		titleStyle.Render("gitm8"),
		keyStyle.Render("repo ") + repo,
		keyStyle.Render("branch ") + m.info.Branch,
		keyStyle.Render("user ") + m.info.User,
		keyStyle.Render("upstream ") + upstream,
		keyStyle.Render("sync ") + sync,
		keyStyle.Render("staged ") + strconv.Itoa(m.info.Staged),
		keyStyle.Render("unstaged ") + strconv.Itoa(m.info.Unstaged),
		keyStyle.Render("last pull ") + lastPull,
		keyStyle.Render("viewer ") + m.mode,
	}
	return panelStyle.Width(m.panelWidth()).Render(strings.Join(items, "  "))
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

	lines := []string{titleStyle.Render("Files"), mutedStyle.Render("↑/↓ preview  enter edit")}
	end := min(len(files), m.fileOffset+visibleRows)
	for i := m.fileOffset; i < end; i++ {
		file := files[i]
		pointer := "  "
		style := lipgloss.NewStyle()
		if i == m.fileCursor {
			style = activeStyle
			pointer = keyStyle.Render("> ")
		}
		badge := statusBadge(file)
		lines = append(lines, pointer+badge+" "+style.Render(trimMiddle(fileListName(file), width-9)))
	}
	if len(m.files) == 0 {
		lines = append(lines, mutedStyle.Render("No changed files"))
	} else if len(files) == 0 {
		lines = append(lines, mutedStyle.Render("No changed files match the current filter"))
	}
	if len(files) > visibleRows {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d-%d of %d", m.fileOffset+1, end, len(files))))
	}

	return panelStyle.Width(width).Height(panelHeight).Render(strings.Join(lines, "\n"))
}

// reviewPanel renders the active viewer title and viewport content.
func (m Model) reviewPanel(height int) string {
	reviewWidth := max(20, m.width-m.filesWidth()-4)
	reviewHeight := max(1, height-2)
	title := titleStyle.Render(viewerTitle(m.mode))
	return panelStyle.Width(reviewWidth).Height(reviewHeight).Render(title + "\n" + m.review.View())
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
		filterLabel := "filter files"
		if m.mode == "preview" {
			filterLabel = "find in file"
		}
		return []string{strings.Join([]string{
			keyStyle.Render("[h]") + " all keys",
			keyStyle.Render("[↑/↓]") + " files",
			keyStyle.Render("[enter]") + " edit",
			keyStyle.Render("[/]") + " " + filterLabel,
			keyStyle.Render("[c]") + " commit",
			keyStyle.Render("[P]") + " push",
			keyStyle.Render("[q]") + " quit",
		}, "  ")}
	}
}

// Small View Helpers

// statusBadge turns Git status values into short staged/unstaged labels.
func statusBadge(file git.FileStatus) string {
	switch {
	case file.Renamed():
		return keyStyle.Render("REN")
	case file.Deleted():
		return errorStyle.Render("DEL")
	case file.Staged() && file.Unstaged():
		return activeStyle.Render("S/U")
	case file.Staged():
		return keyStyle.Render("S  ")
	case file.Unstaged():
		return mutedStyle.Render(" U ")
	default:
		return mutedStyle.Render(file.Label())
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

// helpView renders the in-app reference for keys, config, profiles, and docs.
func (m Model) helpView() string {
	var b strings.Builder
	b.WriteString("Press esc or h to return  ·  j/k to scroll\n\n")

	b.WriteString(titleStyle.Render("KEYS") + "\n")
	commitHelp := "commit staged changes"
	if m.config.AIAvailable {
		commitHelp = "commit staged changes (ctrl+g generates a message in commit mode)"
	}
	pushHelp := "pull (--ff-only) / push (offers force-with-lease if rejected)"
	if m.config.MattMode {
		pushHelp = "pull (--ff-only) / push (matt_mode: always --force, no safety net)"
	}
	keys := [][2]string{
		{"↑/↓", "move file selection (previews it)"},
		{"enter", "open the viewed file in GITM8_EDITOR"},
		{"0", "back to the repo-wide code review"},
		{"d", "toggle the selected file between diff and contents"},
		{"/", "search and highlight inside the viewed file contents"},
		{"x", "discard all changes to selected file"},
		{"s / S", "stage selected file / stage all"},
		{"u / U", "unstage selected file / unstage all"},
		{"n", "stash selected file"},
		{"c", commitHelp},
		{"f", "fetch (--all --prune)"},
		{"p / P", pushHelp},
		{"z", "mark commits, choose a base, squash in-TUI (no editor)"},
		{"C", "conflict mode for unmerged files"},
		{"t", "stash panel"},
		{"b", "branch switcher"},
		{"W", "in branch switcher: switch and bring current changes"},
		{"l", "view recent commit logs"},
		{"i", "identity switcher (git user profiles)"},
		{"r", "pull request options"},
		{"v", "list and create GitHub releases"},
		{"R", "rebase the current branch onto another"},
		{"h", "this help"},
		{"o", "expand or collapse the git output box"},
		{"tab", "hide or show the footer key bar"},
		{"y", "open the yazi file manager"},
		{"j/k", "scroll the viewer line by line"},
		{"pgdn/pgup", "scroll the viewer by a page"},
		{"g / G", "jump viewer to top / bottom"},
		{"q / ctrl+c", "quit"},
	}
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s  %s\n", keyStyle.Render(fmt.Sprintf("%-11s", k[0])), k[1])
	}

	b.WriteString("\n" + titleStyle.Render("CONFIG") + "\n")
	b.WriteString("  Loaded from ~/.gitm8/.gitm8rc, ~/.gitm8/gitm8rc, legacy ~/.gitm8rc, then credentials.\n")
	b.WriteString("  Env vars override defaults.\n")
	b.WriteString("  Themes: " + strings.Join(themeNames(), ", ") + ".\n")
	for _, line := range []string{
		"GITM8_DEFAULT_BRANCH", "GITM8_EDITOR", "GITM8_THEME",
		"GITM8_CONFIRM_DESTRUCTIVE_ACTIONS", "GITM8_FETCH_ON_STARTUP",
		"GITM8_SHOW_COMMIT_GRAPH", "GITM8_AI_PROVIDER",
		"GITM8_OLLAMA_URL",
	} {
		fmt.Fprintf(&b, "    %s\n", line)
	}

	b.WriteString("\n" + titleStyle.Render("PROFILES") + "\n")
	b.WriteString("  Define identities in ~/.gitm8/profiles, one per line:\n")
	b.WriteString("    Work = Ada Lovelace <ada@work.example>\n")
	b.WriteString("  Press i to switch; sets git user for this repo only.\n")

	b.WriteString("\n" + titleStyle.Render("RELEASES") + "\n")
	b.WriteString("  v             open GitHub releases\n")
	b.WriteString("  ↑/↓ or j/k    choose a release\n")
	b.WriteString("  enter         inspect notes, metadata, and assets\n")
	b.WriteString("  n             create a release\n")
	b.WriteString("  r             refresh releases\n")
	b.WriteString("  ctrl+d        toggle draft while creating\n")
	b.WriteString("  ctrl+p        toggle prerelease while creating\n")
	b.WriteString("  ctrl+g        toggle generated notes while creating\n")
	b.WriteString("  esc           return\n")

	b.WriteString("\n" + titleStyle.Render("DOCS") + "\n")
	b.WriteString("  See README.md for full documentation.\n")
	return b.String()
}

// helpScreenView gives the complete key reference the full terminal instead of
// squeezing it beside the changed-files panel.
func (m Model) helpScreenView() string {
	contentWidth := max(40, m.width-4)
	header := panelStyle.Width(contentWidth).Render(strings.Join([]string{
		titleStyle.Render("gitm8") + "  " + keyStyle.Render("KEYBOARD REFERENCE"),
		mutedStyle.Render("Every command, in one place"),
	}, "\n"))
	footer := mutedStyle.Width(max(20, m.width)).Render(strings.Join([]string{
		keyStyle.Render("[j/k]") + " scroll",
		keyStyle.Render("[pgup/pgdn]") + " page",
		keyStyle.Render("[g/G]") + " top/bottom",
		keyStyle.Render("[esc/h]") + " close",
	}, "  "))
	bodyHeight := max(8, m.height-lipgloss.Height(header)-lipgloss.Height(footer)-1)
	m.review.Width = max(20, contentWidth-4)
	m.review.Height = max(1, bodyHeight-4)
	body := panelStyle.Width(contentWidth).Height(max(1, bodyHeight-2)).Render(m.review.View())
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
