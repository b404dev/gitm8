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
	if m.splash {
		return m.splashView()
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
		parts = append(parts, commitBoxStyle(m.width).Render(titleStyle.Render("Commit message")+"\n"+m.commit.View()+"\n"+mutedStyle.Render("ctrl+g: generate  enter: commit  esc: cancel")))
	}
	if m.mode == "new-branch" {
		parts = append(parts, commitBoxStyle(m.width).Render(titleStyle.Render("Create branch")+"\n"+m.branchInput.View()+"\n"+mutedStyle.Render("enter: create  esc: cancel")))
	}
	if m.mode == "delete-branch" {
		parts = append(parts, commitBoxStyle(m.width).Render(titleStyle.Render("Delete branch")+"\n"+m.notice+"\n"+mutedStyle.Render("l: local  r: local + remote  esc: cancel")))
	}
	return strings.Join(parts, "\n")
}

// outputBar renders compact or expanded Git command output.
func (m Model) outputBar() string {
	if m.loading {
		return panelStyle.Width(m.panelWidth()).Render(keyStyle.Render("git ") + m.spinner.View() + " " + mutedStyle.Render("working..."))
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
		return panelStyle.Width(m.panelWidth()).Render(keyStyle.Render("git ") + style.Render(trimMiddle(compact, contentWidth)))
	}

	lines, truncated := visibleOutputLines(output, contentWidth, m.outputMaxLines())
	var b strings.Builder
	b.WriteString(keyStyle.Render("git"))
	if truncated > 0 {
		fmt.Fprintf(&b, " %s", mutedStyle.Render(fmt.Sprintf("(%d earlier lines hidden)", truncated)))
	}
	b.WriteString("\n")
	b.WriteString(style.Render(strings.Join(lines, "\n")))
	return panelStyle.Width(m.panelWidth()).Render(b.String())
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
	width := m.filesWidth()
	panelHeight := max(1, height-2)
	visibleRows := max(1, panelHeight-4)

	lines := []string{titleStyle.Render("Files"), mutedStyle.Render("↑/↓ select  enter preview")}
	end := min(len(m.files), m.fileOffset+visibleRows)
	for i := m.fileOffset; i < end; i++ {
		file := m.files[i]
		pointer := "  "
		style := lipgloss.NewStyle()
		if i == m.fileCursor {
			style = activeStyle
			pointer = keyStyle.Render("> ")
		}
		badge := statusBadge(file)
		lines = append(lines, pointer+badge+" "+style.Render(trimMiddle(file.Path, width-9)))
	}
	if len(m.files) == 0 {
		lines = append(lines, mutedStyle.Render("No changed files"))
	}
	if len(m.files) > visibleRows {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d-%d of %d", m.fileOffset+1, end, len(m.files))))
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
	rowOne := []string{
		keyStyle.Render("[↑/↓]") + " files",
		keyStyle.Render("[enter]") + " preview",
		keyStyle.Render("[0]") + " repo",
		keyStyle.Render("[s]") + " stage",
		keyStyle.Render("[S]") + " stage all",
		keyStyle.Render("[u]") + " unstage",
		keyStyle.Render("[U]") + " unstage all",
		keyStyle.Render("[c]") + " commit",
	}
	rowTwo := []string{
		keyStyle.Render("[d]") + " diff for target",
		keyStyle.Render("[f]") + " fetch",
		keyStyle.Render("[p]") + " pull",
		keyStyle.Render("[P]") + " push",
		keyStyle.Render("[x]") + " sync",
		keyStyle.Render("[b]") + " branches",
		keyStyle.Render("[l]") + " logs",
		keyStyle.Render("[i]") + " identity",
		keyStyle.Render("[y]") + " yazi",
		keyStyle.Render("[r/ctrl+p]") + " PR",
		keyStyle.Render("[R/ctrl+r]") + " rebase",
		keyStyle.Render("[h]") + " help",
		keyStyle.Render("[o]") + " output",
		keyStyle.Render("[tab]") + " hide bar",
		keyStyle.Render("[j/k]") + " viewer scroll",
		keyStyle.Render("[q]") + " quit",
	}
	return mutedStyle.Width(max(20, m.width)).Render(strings.Join(rowOne, "  ") + "\n" + strings.Join(rowTwo, "  "))
}

// Small View Helpers

// statusBadge turns Git status values into short staged/unstaged labels.
func statusBadge(file git.FileStatus) string {
	switch {
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
	case "logs":
		return "Commit Logs"
	case "profiles":
		return "Switch Identity"
	case "help":
		return "Help"
	case "new-branch":
		return "Create Branch"
	case "delete-branch":
		return "Delete Branch"
	case "commit":
		return "Commit"
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

// Help View

// helpView renders the in-app reference for keys, config, profiles, and docs.
func (m Model) helpView() string {
	var b strings.Builder
	b.WriteString("Press esc or h to return  ·  j/k to scroll\n\n")

	b.WriteString(titleStyle.Render("KEYS") + "\n")
	keys := [][2]string{
		{"↑/↓", "move file selection (previews it)"},
		{"enter", "preview the selected file"},
		{"0", "back to the repo-wide code review"},
		{"d", "toggle the selected file between diff and contents"},
		{"s / S", "stage selected file / stage all"},
		{"u / U", "unstage selected file / unstage all"},
		{"c", "commit staged changes (ctrl+g generates a message in commit mode)"},
		{"f", "fetch (--all --prune)"},
		{"p / P", "pull (--ff-only) / push"},
		{"x", "sync: pull --rebase, then push"},
		{"b", "branch switcher"},
		{"W", "in branch switcher: switch and bring current changes"},
		{"l", "view recent commit logs"},
		{"i", "identity switcher (git user profiles)"},
		{"r / ctrl+p", "create or show pull request for current branch"},
		{"R / ctrl+r", "rebase the current branch onto another"},
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
	b.WriteString("  Loaded from ~/.gitm8rc then ~/.gitm8/credentials.\n")
	b.WriteString("  Env vars override defaults.\n")
	b.WriteString("  Themes: " + strings.Join(themeNames(), ", ") + ".\n")
	for _, line := range []string{
		"GITM8_DEFAULT_BRANCH", "GITM8_EDITOR", "GITM8_THEME",
		"GITM8_CONFIRM_DESTRUCTIVE_ACTIONS", "GITM8_FETCH_ON_STARTUP",
		"GITM8_SHOW_COMMIT_GRAPH",
	} {
		fmt.Fprintf(&b, "    %s\n", line)
	}

	b.WriteString("\n" + titleStyle.Render("PROFILES") + "\n")
	b.WriteString("  Define identities in ~/.gitm8/profiles, one per line:\n")
	b.WriteString("    Work = Ada Lovelace <ada@work.example>\n")
	b.WriteString("  Press i to switch; sets git user for this repo only.\n")

	b.WriteString("\n" + titleStyle.Render("DOCS") + "\n")
	b.WriteString("  See README.md for full documentation.\n")
	return b.String()
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
