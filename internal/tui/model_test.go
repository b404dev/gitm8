package tui

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/b404dev/gitm8/internal/git"
)

// TestVisibleOutputLinesWrapsLongLines checks long Git output wraps cleanly.
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

// TestRebaseViewShowsLocalRebaseControls checks rebase help lists continue/abort/skip.
func TestRebaseViewShowsLocalRebaseControls(t *testing.T) {
	m := Model{height: 24, branches: []string{"main", "feature"}, info: git.RepoInfo{Branch: "feature"}}
	got := m.rebaseView()
	if !strings.Contains(got, "c continue") || !strings.Contains(got, "a abort") || !strings.Contains(got, "s skip") {
		t.Fatalf("rebaseView() = %q, want local rebase controls", got)
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
