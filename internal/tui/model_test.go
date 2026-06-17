package tui

import (
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/b404dev/gitm8/internal/config"
	"github.com/b404dev/gitm8/internal/git"
)

// TestVisibleOutputLinesWrapsLongLines checks long Git output wraps cleanly
func TestVisibleOutputLinesWrapsLongLines(t *testing.T) {
	got, hidden := visibleOutputLines("alpha beta gamma", 8, 10)
	want := []string{"alpha", "beta", "gamma"}
	if hidden != 0 {
		t.Fatalf("hidden = %d, want 0", hidden)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visibleOutputLines() = %#v, want %#v", got, want)
	}
}

// TestVisibleOutputLinesKeepsMostRecentLines checks the newest output stays visible.
func TestVisibleOutputLinesKeepsMostRecentLines(t *testing.T) {
	got, hidden := visibleOutputLines("one\ntwo\nthree\nfour", 20, 2)
	want := []string{"three", "four"}
	if hidden != 2 {
		t.Fatalf("hidden = %d, want 2", hidden)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visibleOutputLines() = %#v, want %#v", got, want)
	}
}

// TestOutputMaxLinesExpands checks expanded output shows more history.
func TestOutputMaxLinesExpands(t *testing.T) {
	collapsed := Model{height: 30}
	expanded := Model{height: 30, outputExpanded: true}
	if expanded.outputMaxLines() <= collapsed.outputMaxLines() {
		t.Fatalf("expanded outputMaxLines = %d, want more than %d", expanded.outputMaxLines(), collapsed.outputMaxLines())
	}
}

// TestOutputBarCollapsedStaysSingleLine checks collapsed output stays compact.
func TestOutputBarCollapsedStaysSingleLine(t *testing.T) {
	m := Model{width: 80, gitOutput: "one\ntwo\nthree"}
	if got := lipgloss.Height(m.outputBar()); got != 3 {
		t.Fatalf("collapsed outputBar height = %d, want 3", got)
	}
}

// TestFooterCanBeHidden checks the footer toggle can reclaim screen space.
func TestFooterCanBeHidden(t *testing.T) {
	m := Model{width: 80, footerHidden: true}
	if got := m.footer(); got != "" {
		t.Fatalf("hidden footer = %q, want empty", got)
	}
}

func TestFooterIsContextSensitive(t *testing.T) {
	branches := Model{width: 80, mode: "branches"}
	branchFooter := branches.footer()
	if !strings.Contains(branchFooter, "switch with changes") || !strings.Contains(branchFooter, "create") || strings.Contains(branchFooter, "stage all") {
		t.Fatalf("branches footer = %q, want branch actions and no dashboard noise", branchFooter)
	}

	preview := Model{width: 80, mode: "preview"}
	previewFooter := preview.footer()
	if !strings.Contains(previewFooter, "find in file") {
		t.Fatalf("preview footer = %q, want file-search shortcut", previewFooter)
	}

	review := Model{width: 80, mode: "review"}
	reviewFooter := review.footer()
	if !strings.Contains(reviewFooter, "filter files") {
		t.Fatalf("review footer = %q, want changed-files filter shortcut", reviewFooter)
	}
}

// TestSplashViewShowsStartupContent checks the splash screen has the expected text.
func TestSplashViewShowsStartupContent(t *testing.T) {
	m := Model{width: 80, height: 24, ready: true, splash: true, splashFrame: 3, splashMessage: "thinking about rebase"}
	got := m.splashView()
	if !strings.Contains(got, "██████") || !strings.Contains(got, "fetch • branch • commit • push") || !strings.Contains(got, "preparing repository view") || !strings.Contains(got, "thinking about rebase") {
		t.Fatalf("splashView() = %q, want startup content", got)
	}
}

// TestBranchesViewShowsSwitchWithChangesKey checks branch help mentions W.
func TestBranchesViewShowsSwitchWithChangesKey(t *testing.T) {
	got := branchesView([]string{"main"}, 0, 0, 5)
	if !strings.Contains(got, "W to switch with changes") {
		t.Fatalf("branchesView() = %q, want switch-with-changes hint", got)
	}
}

func TestFilteredFilesMatchesPathsAndLabels(t *testing.T) {
	m := Model{
		files: []git.FileStatus{
			{Path: "cmd/main.go", Index: 'M', Worktree: ' '},
			{Path: "docs/guide.md", Index: ' ', Worktree: 'M'},
			{Path: "old.txt", OldPath: "renamed.txt", Index: 'R', Worktree: ' '},
		},
	}

	if got := m.filteredFiles(); len(got) != 3 {
		t.Fatalf("filteredFiles() without query = %d, want 3", len(got))
	}

	m.fileFilter.SetValue("guide")
	got := m.filteredFiles()
	if len(got) != 1 || got[0].Path != "docs/guide.md" {
		t.Fatalf("filteredFiles() = %#v, want docs/guide.md", got)
	}

	m.fileFilter.SetValue("ren*")
	got = m.filteredFiles()
	if len(got) != 1 || got[0].Path != "old.txt" {
		t.Fatalf("filteredFiles() wildcard = %#v, want renamed row", got)
	}
}

func TestFileListNameShowsBasenameOnly(t *testing.T) {
	cases := []struct {
		file git.FileStatus
		want string
	}{
		{file: git.FileStatus{Path: "cmd/main.go"}, want: "main.go"},
		{file: git.FileStatus{Path: "assets/icons", Directory: true}, want: "icons/"},
		{file: git.FileStatus{Path: "new.txt", OldPath: "old.txt"}, want: "old.txt -> new.txt"},
	}
	for _, tc := range cases {
		if got := fileListName(tc.file); got != tc.want {
			t.Fatalf("fileListName(%#v) = %q, want %q", tc.file, got, tc.want)
		}
	}
}

func TestOutputBarShowsSelectedFullPath(t *testing.T) {
	m := Model{width: 120, mode: "preview", target: "src/deep/main.go"}
	if got := m.outputBar(); !strings.Contains(got, "path src/deep/main.go") {
		t.Fatalf("outputBar() = %q, want selected full path", got)
	}
	if got := m.topBar(); strings.Contains(got, "path src/deep/main.go") {
		t.Fatalf("topBar() = %q, want path only in output bar", got)
	}
}

func TestStageNoticeNamesDeletedPaths(t *testing.T) {
	file := git.FileStatus{Path: "old.txt", Worktree: 'D'}
	if got := stageNotice(file); got != "Staged removal of old.txt" {
		t.Fatalf("stageNotice() = %q, want deletion-specific text", got)
	}
}

// TestHandleCommitMessageGeneratedPopulatesInput checks generated text stays in
// the commit prompt instead of replacing the review panel.
func TestHandleCommitMessageGeneratedPopulatesInput(t *testing.T) {
	m := New(git.Runner{}, config.Config{Theme: "default"})
	m.loading = true
	m.mode = "commit"

	got := m.handleCommitMessageGenerated(commitMessageGeneratedMsg{message: "Update commit flow"})
	if got.loading {
		t.Fatal("loading = true, want false")
	}
	if got.mode != "commit" {
		t.Fatalf("mode = %q, want commit", got.mode)
	}
	if got.commit.Value() != "Update commit flow" {
		t.Fatalf("commit value = %q, want generated message", got.commit.Value())
	}
}

// TestPullRequestViewShowsAIUnavailable keeps manual PR creation available.
func TestPullRequestViewShowsAIUnavailable(t *testing.T) {
	m := Model{config: config.Config{AIProvider: "codex", AIAvailable: false, AIUnavailableReason: "codex is not installed or not on PATH"}}

	got := m.pullRequestView()
	if !strings.Contains(got, "AI generation unavailable: codex is not installed or not on PATH") {
		t.Fatalf("pullRequestView() = %q, want unavailable reason", got)
	}
	if strings.Contains(got, "g  generate") {
		t.Fatalf("pullRequestView() = %q, should not offer generated PR text", got)
	}
	if !strings.Contains(got, "m  write it yourself") {
		t.Fatalf("pullRequestView() = %q, want manual PR option", got)
	}
}

// TestCommitPromptHelpHidesGenerateWhenAIUnavailable avoids advertising ctrl+g.
func TestCommitPromptHelpHidesGenerateWhenAIUnavailable(t *testing.T) {
	m := Model{config: config.Config{AIAvailable: false}}
	if got := m.commitPromptHelp(); strings.Contains(got, "ctrl+g") {
		t.Fatalf("commitPromptHelp() = %q, should not advertise AI generate", got)
	}

	m.config.AIAvailable = true
	if got := m.commitPromptHelp(); !strings.Contains(got, "ctrl+g") {
		t.Fatalf("commitPromptHelp() = %q, want AI generate hint", got)
	}
}

// TestHelpViewIncludesSquashKey documents the dashboard squash workflow.
func TestHelpViewIncludesSquashKey(t *testing.T) {
	m := Model{config: config.Config{AIAvailable: true}}
	got := m.helpView()
	if !strings.Contains(got, "mark commits, choose a base") {
		t.Fatalf("helpView() = %q, want squash key help", got)
	}
}

// TestHelpViewIncludesDiscardKey documents the guarded discard workflow.
func TestHelpViewIncludesDiscardKey(t *testing.T) {
	m := Model{}
	got := m.helpView()
	if !strings.Contains(got, "discard all changes to selected file") {
		t.Fatalf("helpView() = %q, want discard key help", got)
	}
}

func TestHelpViewIncludesFindKey(t *testing.T) {
	m := Model{}
	got := m.helpView()
	if !strings.Contains(got, "search and highlight inside the viewed file contents") {
		t.Fatalf("helpView() = %q, want find key help", got)
	}
}

func TestSearchLinesRequireFullWordByDefault(t *testing.T) {
	content := "main.go\n\n  1  package main\n  2  func renderSearchView() string\n  3  render nil\n"
	got := fuzzySearchLines(content, "render", 10)
	if len(got) == 0 {
		t.Fatal("fuzzySearchLines() returned no matches")
	}
	if got[0].LineNo != 3 {
		t.Fatalf("first match = %#v, want full-word line 3", got[0])
	}
	if len(got) != 1 {
		t.Fatalf("matches = %#v, want only full-word render", got)
	}
}

func TestSearchLinesSupportWildcardWords(t *testing.T) {
	content := "  1  fuzzy searching\n  2  find selected zebra\n"
	got := fuzzySearchLines(content, "search*", 10)
	if len(got) == 0 {
		t.Fatal("fuzzySearchLines() returned no matches")
	}
	if got[0].LineNo != 1 {
		t.Fatalf("first match line = %d, want substring line 1", got[0].LineNo)
	}
}

func TestSearchOutputBarShowsInputAndKeys(t *testing.T) {
	m := New(git.Runner{}, config.Config{Theme: "default"})
	m.width = 100
	m.mode = "search"
	m.searchInput.SetValue("render*")
	m.searchMatches = []searchMatch{{LineNo: 2}}

	got := m.outputBar()
	if !strings.Contains(got, "/ render*") || !strings.Contains(got, "1/1 matches") || !strings.Contains(got, "↑ prev") || !strings.Contains(got, "↓ next") {
		t.Fatalf("outputBar() = %q, want search input, summary, and keys", got)
	}
}

func TestSearchViewKeepsFileContextAndHighlightsMatch(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	applyTheme("default")
	m := New(git.Runner{}, config.Config{Theme: "default"})
	m.target = "main.go"
	m.viewerContent = "main.go\n\n  1  package main\n  2  func renderSearchView() string\n  3  return nil\n"
	m.searchInput.SetValue("render*")
	m.refreshSearchResults()

	got := m.searchView()
	if strings.Contains(got, "/ render*") {
		t.Fatalf("searchView() = %q, should not include search prompt", got)
	}
	if !strings.Contains(got, "package main") || !strings.Contains(got, "return nil") {
		t.Fatalf("searchView() = %q, want surrounding file context", got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("searchView() = %q, want ANSI highlighting", got)
	}
}

// TestEditorCommandUsesShellForEditorWithArgs keeps configured editor commands usable.
func TestEditorCommandUsesShellForEditorWithArgs(t *testing.T) {
	cmd, err := editorCommand("code --wait", "main.go")
	if err != nil {
		t.Fatalf("editorCommand() error = %v", err)
	}
	if runtime.GOOS == "windows" {
		if len(cmd.Args) < 4 || cmd.Args[0] != "cmd" || cmd.Args[1] != "/C" {
			t.Fatalf("editorCommand args = %#v, want cmd command", cmd.Args)
		}
		return
	}
	if len(cmd.Args) < 4 || cmd.Args[0] != "sh" || cmd.Args[1] != "-c" {
		t.Fatalf("editorCommand args = %#v, want shell command", cmd.Args)
	}
	if cmd.Args[len(cmd.Args)-1] != "main.go" {
		t.Fatalf("editorCommand file arg = %q, want main.go", cmd.Args[len(cmd.Args)-1])
	}
}

// TestRebaseViewShowsLocalRebaseControls checks rebase help lists continue/abort/skip.
func TestRebaseViewShowsLocalRebaseControls(t *testing.T) {
	m := Model{height: 24, branches: []string{"main", "feature"}, info: git.RepoInfo{Branch: "feature"}}
	got := m.rebaseView()
	if !strings.Contains(got, "c continue") || !strings.Contains(got, "a abort") || !strings.Contains(got, "s skip") {
		t.Fatalf("rebaseView() = %q, want local rebase controls", got)
	}
}

// TestConflictsViewShowsResolveControls checks conflict mode documents its actions.
func TestConflictsViewShowsResolveControls(t *testing.T) {
	m := Model{height: 24, conflicts: []string{"main.go"}}
	got := m.conflictsView("Conflict markers:\n3: <<<<<<< HEAD\n", "main.go\n\n1  <<<<<<< HEAD\n")
	if !strings.Contains(got, "m to mark resolved") || !strings.Contains(got, "enter opens the file") || !strings.Contains(got, "3: <<<<<<< HEAD") {
		t.Fatalf("conflictsView() = %q, want conflict controls and file", got)
	}
}

// TestStashesViewShowsStashActions checks stash mode documents its actions.
func TestStashesViewShowsStashActions(t *testing.T) {
	m := Model{height: 24, stashes: []git.Stash{{Ref: "stash@{0}", Subject: "WIP on main"}}}
	got := m.stashesView("diff --git a/main.go b/main.go\n")
	if !strings.Contains(got, "a apply") || !strings.Contains(got, "p pop") || !strings.Contains(got, "D drop") {
		t.Fatalf("stashesView() = %q, want stash controls", got)
	}
}

// TestThemeNamesAreSorted checks theme names are stable and easy to scan.
func TestThemeNamesAreSorted(t *testing.T) {
	got := themeNames()
	if !sort.StringsAreSorted(got) {
		t.Fatalf("themeNames() = %#v, want sorted", got)
	}
	if len(got) != len(palettes) {
		t.Fatalf("themeNames length = %d, want %d", len(got), len(palettes))
	}
}

// TestCatppuccinDefaultUsesMocha checks the catppuccin theme alias.
func TestCatppuccinDefaultUsesMocha(t *testing.T) {
	if palettes["catppuccin"] != palettes["catppuccin-mocha"] {
		t.Fatal("catppuccin should alias catppuccin-mocha")
	}
}

// TestHighlightPreviewStylesNumberedCode checks highlighting keeps the original text.
func TestHighlightPreviewStylesNumberedCode(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	applyTheme("catppuccin")
	got := highlightPreview("main.go", "main.go\n\n1  func main() {\n2  \treturn\n")
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("highlightPreview() = %q, want ANSI styling", got)
	}
	if !strings.Contains(got, "func") || !strings.Contains(got, "return") {
		t.Fatalf("highlightPreview() = %q, want original code text preserved", got)
	}
}

// TestCommentStartIndexIgnoresCommentPrefixInsideString checks URLs are not treated as comments.
func TestCommentStartIndexIgnoresCommentPrefixInsideString(t *testing.T) {
	line := `fmt.Println("https://example.test") // real comment`
	got := commentStartIndex("main.go", line)
	want := strings.Index(line, " // real comment") + 1
	if got != want {
		t.Fatalf("commentStartIndex() = %d, want %d", got, want)
	}
}
