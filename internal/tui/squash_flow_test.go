package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/config"
	"github.com/b404dev/gitm8/internal/git"
)

// TestSquashFlowSelectAllCollapsesBranch drives the real Update loop the way the
// app does: open the squash picker (z), mark every commit (a), choose a base,
// apply (enter), and confirm the branch collapses to a single commit.
func TestSquashFlowSelectAllCollapsesBranch(t *testing.T) {
	dir := newFlowRepo(t, 3)
	m := newFlowModel(t, dir)

	m = drive(t, m, key("z"))
	if m.mode != "squash" {
		t.Fatalf("after z, mode = %q, want squash (gitOutput=%q)", m.mode, m.gitOutput)
	}
	if len(m.commits) != 3 {
		t.Fatalf("squash candidates = %d, want 3", len(m.commits))
	}

	m = drive(t, m, key("a"))
	if m.squashFoldCount() != 3 {
		t.Fatalf("fold count after select-all = %d, want 3", m.squashFoldCount())
	}

	m = drive(t, m, key("B"))
	if m.squashFoldCount() != 2 {
		t.Fatalf("fold count after picking base = %d, want 2", m.squashFoldCount())
	}

	m.loading = true // make m.action return a bare command so the driver is deterministic
	m = drive(t, m, enter())

	if m.err != nil {
		t.Fatalf("squash returned error: %v", m.err)
	}
	if got := countCommitsAhead(t, dir); got != 1 {
		t.Fatalf("commits ahead of main after squash = %d, want 1", got)
	}
}

// TestSquashFlowCanMoveBase confirms the base can be moved away from the newest
// commit before marking another commit to squash into it.
func TestSquashFlowCanMoveBase(t *testing.T) {
	dir := newFlowRepo(t, 3)
	m := newFlowModel(t, dir)

	m = drive(t, m, key("z"))
	if m.mode != "squash" {
		t.Fatalf("after z, mode = %q, want squash (gitOutput=%q)", m.mode, m.gitOutput)
	}

	// commits are newest-first: mark the middle commit first, then choose the
	// oldest commit as the base.
	m = drive(t, m, down())
	m = drive(t, m, key("S"))
	m = drive(t, m, down())
	m = drive(t, m, key("B"))
	if m.squashFoldCount() != 1 {
		t.Fatalf("fold count = %d, want 1", m.squashFoldCount())
	}

	m.loading = true
	m = drive(t, m, enter())

	if m.err != nil {
		t.Fatalf("squash returned error: %v", m.err)
	}
	if got := countCommitsAhead(t, dir); got != 2 {
		t.Fatalf("commits ahead of main after squash = %d, want 2", got)
	}
}

func TestStashPanelCanCreateStash(t *testing.T) {
	dir := newFlowRepo(t, 0)
	m := newFlowModel(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "scratch.txt"), []byte("scratch\n"), 0o644); err != nil {
		t.Fatalf("write scratch: %v", err)
	}

	m = drive(t, m, key("t"))
	if m.mode != "stashes" {
		t.Fatalf("after t, mode = %q, want stashes", m.mode)
	}

	m = drive(t, m, key("n"))
	if m.err != nil {
		t.Fatalf("stash action returned error: %v", m.err)
	}
	if len(m.stashes) != 1 {
		t.Fatalf("stashes after n = %d, want 1", len(m.stashes))
	}
	if strings.TrimSpace(m.notice) == "" || !strings.Contains(m.gitOutput, "Saved working directory") {
		t.Fatalf("notice=%q gitOutput=%q, want stash output preserved", m.notice, m.gitOutput)
	}
	if got := strings.TrimSpace(flowGit(t, dir, "status", "--short")); got != "" {
		t.Fatalf("status after stash = %q, want clean worktree", got)
	}
}

func TestViewerCanStashSelectedFileOnly(t *testing.T) {
	dir := newFlowRepo(t, 0)
	m := newFlowModel(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "viewer.txt"), []byte("viewer\n"), 0o644); err != nil {
		t.Fatalf("write viewer file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatalf("write other file: %v", err)
	}
	m = drive(t, m, repoLoadedMsg{
		info: m.info,
		files: []git.FileStatus{
			{Path: "viewer.txt", Index: '?', Worktree: '?'},
			{Path: "other.txt", Index: '?', Worktree: '?'},
		},
		review: "viewer",
		target: "viewer.txt",
		mode:   "preview",
	})

	m = drive(t, m, key("n"))
	if m.err != nil {
		t.Fatalf("viewer stash returned error: %v", m.err)
	}
	stashes := strings.TrimSpace(flowGit(t, dir, "stash", "list"))
	if !strings.Contains(stashes, "gitm8 stash") {
		t.Fatalf("stash list = %q, want gitm8 stash", stashes)
	}
	got := strings.TrimSpace(flowGit(t, dir, "status", "--short"))
	if strings.Contains(got, "viewer.txt") || !strings.Contains(got, "other.txt") {
		t.Fatalf("status after selected-file stash = %q, want only other.txt remaining", got)
	}
	if strings.TrimSpace(m.notice) == "" || !strings.Contains(m.gitOutput, "Saved working directory") {
		t.Fatalf("notice=%q gitOutput=%q, want stash output preserved", m.notice, m.gitOutput)
	}
}

// drive sends one message into Update, then runs the resulting command and feeds
// its message back, repeating until the model settles. Spinner ticks are
// dropped so the loop terminates.
func drive(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	model, cmd := m.Update(msg)
	m = model.(Model)
	for _, next := range runCmd(t, cmd) {
		m = drive(t, m, next)
	}
	return m
}

// runCmd executes a command and returns the messages it produced, flattening
// batches and discarding spinner ticks (which would otherwise never settle).
func runCmd(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch v := msg.(type) {
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range v {
			out = append(out, runCmd(t, c)...)
		}
		return out
	case spinner.TickMsg:
		return nil
	case nil:
		return nil
	default:
		return []tea.Msg{msg}
	}
}

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func enter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }
func down() tea.KeyMsg  { return tea.KeyMsg{Type: tea.KeyDown} }
func up() tea.KeyMsg    { return tea.KeyMsg{Type: tea.KeyUp} }

func newFlowModel(t *testing.T, dir string) Model {
	t.Helper()
	cfg := config.Config{DefaultBranch: "main"}
	m := New(git.NewRunner(dir), cfg)
	m.splash = false
	m.ready = true
	m.width = 120
	m.height = 40
	// Load repo info first, exactly like the app does after the splash, so the
	// dashboard guards see the real branch.
	for _, msg := range runCmd(t, loadDefault(m.runner)) {
		m = drive(t, m, msg)
	}
	m.mode = "review"
	return m
}

func newFlowRepo(t *testing.T, commits int) string {
	t.Helper()
	dir := t.TempDir()
	flowGit(t, "", "init", "-q", dir)
	flowGit(t, dir, "config", "user.name", "Test User")
	flowGit(t, dir, "config", "user.email", "test@example.com")
	flowGit(t, dir, "branch", "-M", "main")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write base: %v", err)
	}
	flowGit(t, dir, "add", "base.txt")
	flowGit(t, dir, "commit", "-qm", "base")
	flowGit(t, dir, "switch", "-q", "-c", "feature")
	for i := 0; i < commits; i++ {
		name := filepath.Join(dir, "f"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(name, []byte("change\n"), 0o644); err != nil {
			t.Fatalf("write change: %v", err)
		}
		flowGit(t, dir, "add", ".")
		flowGit(t, dir, "commit", "-qm", "change "+string(rune('a'+i)))
	}
	return dir
}

func countCommitsAhead(t *testing.T, dir string) int {
	t.Helper()
	out := strings.TrimSpace(flowGit(t, dir, "rev-list", "--count", "main..HEAD"))
	n := 0
	for _, c := range out {
		n = n*10 + int(c-'0')
	}
	return n
}

func flowGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
