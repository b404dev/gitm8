package git

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestNonEmptyLines checks branch and remote list cleanup.
func TestNonEmptyLines(t *testing.T) {
	got := nonEmptyLines(" main\n\n feature \n")
	want := []string{"main", "feature"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nonEmptyLines() = %#v, want %#v", got, want)
	}
}

// TestParseStatusZ checks parsing of `git status --porcelain=v1 -z` output.
func TestParseStatusZ(t *testing.T) {
	cases := []struct {
		name         string
		out          string
		wantPath     string
		wantOldPath  string
		wantIndex    byte
		wantWorktree byte
		wantOK       bool
	}{
		{"worktree only", " M file.txt\x00", "file.txt", "", ' ', 'M', true},
		{"staged", "M  file.txt\x00", "file.txt", "", 'M', ' ', true},
		{"staged and worktree", "MM file.txt\x00", "file.txt", "", 'M', 'M', true},
		{"untracked", "?? new.txt\x00", "new.txt", "", '?', '?', true},
		{"rename", "R  new.txt\x00old.txt\x00", "new.txt", "old.txt", 'R', ' ', true},
		{"path with spaces", " M my file.txt\x00", "my file.txt", "", ' ', 'M', true},
		{"path with newline", " M line\nbreak.txt\x00", "line\nbreak.txt", "", ' ', 'M', true},
		{"too short", " M\x00", "", "", 0, 0, false},
		{"blank", "", "", "", 0, 0, false},
	}
	for _, tc := range cases {
		statuses := parseStatusZ(tc.out)
		ok := len(statuses) > 0
		if ok != tc.wantOK {
			t.Errorf("parseStatusZ(%q) ok = %t, want %t", tc.out, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		got := statuses[0]
		if got.Path != tc.wantPath || got.OldPath != tc.wantOldPath || got.Index != tc.wantIndex || got.Worktree != tc.wantWorktree {
			t.Errorf("parseStatusZ(%q) = {Path:%q OldPath:%q Index:%q Worktree:%q}, want {Path:%q OldPath:%q Index:%q Worktree:%q}",
				tc.out, got.Path, got.OldPath, got.Index, got.Worktree, tc.wantPath, tc.wantOldPath, tc.wantIndex, tc.wantWorktree)
		}
	}
}

// TestStripRemote checks remote branch names are shown cleanly.
func TestStripRemote(t *testing.T) {
	remotes := []string{"origin", "upstream"}
	cases := []struct {
		name       string
		wantName   string
		wantRemote bool
	}{
		{"main", "main", false},
		{"origin/main", "main", true},
		{"origin/feature/login", "feature/login", true},
		{"upstream/dev", "dev", true},
		{"originx/main", "originx/main", false},
	}
	for _, tc := range cases {
		gotName, gotRemote := stripRemote(tc.name, remotes)
		if gotName != tc.wantName || gotRemote != tc.wantRemote {
			t.Errorf("stripRemote(%q) = (%q, %t), want (%q, %t)", tc.name, gotName, gotRemote, tc.wantName, tc.wantRemote)
		}
	}
}

// TestRemoteToWebURL checks common remote URLs become clickable web links.
func TestRemoteToWebURL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://github.com/acme/project.git", "https://github.com/acme/project"},
		{"git@github.com:acme/project.git", "https://github.com/acme/project"},
		{"ssh://git@github.com/acme/project.git", "https://github.com/acme/project"},
		{"https://gitlab.com/team/repo", "https://gitlab.com/team/repo"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := remoteToWebURL(tc.raw); got != tc.want {
			t.Errorf("remoteToWebURL(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// TestCleanCommitSubjectKeepsFirstUsefulLine checks Codex output is reduced to
// the single-line value the commit input expects.
func TestCleanCommitSubjectKeepsFirstUsefulLine(t *testing.T) {
	got := cleanCommitSubject("\n`Update branch deletion docs`\n\nExtra detail\n")
	want := "Update branch deletion docs"
	if got != want {
		t.Fatalf("cleanCommitSubject() = %q, want %q", got, want)
	}
}

// TestParsePullRequestDraft checks the title/body format requested from AI tools.
func TestParsePullRequestDraft(t *testing.T) {
	title, body := parsePullRequestDraft(`{"title":"Add generated PR text","body":"## Summary\n- Added AI PR drafting"}`)
	if title != "Add generated PR text" {
		t.Fatalf("title = %q", title)
	}
	if body != "## Summary\n- Added AI PR drafting" {
		t.Fatalf("body = %q", body)
	}
}

// TestParsePullRequestDraftAllowsJSONFence accepts common fenced model output.
func TestParsePullRequestDraftAllowsJSONFence(t *testing.T) {
	title, body := parsePullRequestDraft("```json\n{\"title\":\"Fix PR flow\",\"body\":\"## Summary\\n- No push\"}\n```")
	if title != "Fix PR flow" || body != "## Summary\n- No push" {
		t.Fatalf("parsePullRequestDraft() = (%q, %q)", title, body)
	}
}

// TestParseStashLine checks the stash list format used by the stash panel.
func TestParseStashLine(t *testing.T) {
	got, ok := parseStashLine("stash@{0}\tWIP on main: abc123 update docs")
	if !ok {
		t.Fatal("parseStashLine() ok = false, want true")
	}
	if got.Ref != "stash@{0}" || got.Subject != "WIP on main: abc123 update docs" {
		t.Fatalf("parseStashLine() = %#v", got)
	}
}

func TestGenerateOllamaTextUsesGenerateAPI(t *testing.T) {
	var gotModel string
	var gotPrompt string
	originalClient := ollamaHTTPClient
	ollamaHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/ps" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"models":[{"name":"qwen2.5-coder:latest","model":"qwen2.5-coder:latest"}]}`)),
				Header:     make(http.Header),
			}, nil
		}
		if r.URL.Path != "/api/generate" {
			t.Fatalf("path = %q, want /api/ps or /api/generate", r.URL.Path)
		}
		var req struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		gotModel = req.Model
		gotPrompt = req.Prompt
		if req.Stream {
			t.Fatal("stream = true, want false")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"response":"Add Ollama support"}`)),
			Header:     make(http.Header),
		}, nil
	})}
	t.Cleanup(func() {
		ollamaHTTPClient = originalClient
	})

	got, err := NewRunner("").GenerateOllamaText(context.Background(), "http://ollama.test", "write commit")
	if err != nil {
		t.Fatalf("GenerateOllamaText() error = %v", err)
	}
	if got != "Add Ollama support" {
		t.Fatalf("GenerateOllamaText() = %q", got)
	}
	if gotModel != "qwen2.5-coder:latest" || gotPrompt != "write commit" {
		t.Fatalf("request = (%q, %q)", gotModel, gotPrompt)
	}
}

func TestGenerateOllamaTextFallsBackToInstalledModel(t *testing.T) {
	var gotModel string
	originalClient := ollamaHTTPClient
	ollamaHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/ps":
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"models":[]}`)),
				Header:     make(http.Header),
			}, nil
		case "/api/tags":
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"models":[{"name":"mistral:latest"}]}`)),
				Header:     make(http.Header),
			}, nil
		case "/api/generate":
			var req struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			gotModel = req.Model
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"response":"Add fallback"}`)),
				Header:     make(http.Header),
			}, nil
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
			return nil, nil
		}
	})}
	t.Cleanup(func() {
		ollamaHTTPClient = originalClient
	})

	got, err := NewRunner("").GenerateOllamaText(context.Background(), "http://ollama.test", "write commit")
	if err != nil {
		t.Fatalf("GenerateOllamaText() error = %v", err)
	}
	if got != "Add fallback" {
		t.Fatalf("GenerateOllamaText() = %q", got)
	}
	if gotModel != "mistral:latest" {
		t.Fatalf("model = %q, want mistral:latest", gotModel)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestPushOutputSetsUpstreamWhenMissing(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	runTestGit(t, "", "init", "-q", "--bare", remote)
	runTestGit(t, dir, "remote", "add", "origin", remote)

	runner := NewRunner(dir)
	if _, err := runner.PushOutput(ctx); err != nil {
		t.Fatalf("PushOutput() error = %v", err)
	}
	upstream := strings.TrimSpace(runTestGit(t, dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"))
	if upstream != "origin/main" {
		t.Fatalf("upstream = %q, want origin/main", upstream)
	}
}

func TestStashCommandsListAndPreviewStash(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("write changed file: %v", err)
	}

	runner := NewRunner(dir)
	if _, err := runner.StashPushOutput(ctx); err != nil {
		t.Fatalf("StashPushOutput() error = %v", err)
	}
	stashes, err := runner.Stashes(ctx)
	if err != nil {
		t.Fatalf("Stashes() error = %v", err)
	}
	if len(stashes) != 1 {
		t.Fatalf("len(stashes) = %d, want 1", len(stashes))
	}
	diff, err := runner.StashDiff(ctx, stashes[0].Ref)
	if err != nil {
		t.Fatalf("StashDiff() error = %v", err)
	}
	if !strings.Contains(diff, "changed") {
		t.Fatalf("StashDiff() = %q, want changed content", diff)
	}
}

func TestConflictFilesListsUnmergedFiles(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	runTestGit(t, dir, "switch", "-q", "-c", "feature")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatalf("write feature file: %v", err)
	}
	runTestGit(t, dir, "commit", "-am", "feature")
	runTestGit(t, dir, "switch", "-q", "main")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("write main file: %v", err)
	}
	runTestGit(t, dir, "commit", "-am", "main")
	runTestGitAllowError(t, dir, "merge", "feature")

	conflicts, err := NewRunner(dir).ConflictFiles(ctx)
	if err != nil {
		t.Fatalf("ConflictFiles() error = %v", err)
	}
	if !reflect.DeepEqual(conflicts, []string{"file.txt"}) {
		t.Fatalf("ConflictFiles() = %#v, want file.txt", conflicts)
	}
}

func TestConflictMarkerReportShowsMarkerLines(t *testing.T) {
	dir := initTestRepo(t)
	content := "start\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> feature\nend\n"
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write conflict file: %v", err)
	}

	got := NewRunner(dir).ConflictMarkerReport("file.txt")
	for _, want := range []string{"2: <<<<<<< HEAD", "4: =======", "6: >>>>>>> feature"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ConflictMarkerReport() = %q, want %q", got, want)
		}
	}
}

func TestDiscardOutputRestoresTrackedFile(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("write changed file: %v", err)
	}

	statuses, err := NewRunner(dir).FileStatuses(ctx)
	if err != nil {
		t.Fatalf("FileStatuses() error = %v", err)
	}
	if _, err := NewRunner(dir).DiscardOutput(ctx, statuses[0]); err != nil {
		t.Fatalf("DiscardOutput() error = %v", err)
	}
	got := strings.TrimSpace(runTestGit(t, dir, "status", "--porcelain", "-uall"))
	if got != "" {
		t.Fatalf("status after discard = %q, want clean", got)
	}
}

func TestDiscardOutputCleansUntrackedFile(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	statuses, err := NewRunner(dir).FileStatuses(ctx)
	if err != nil {
		t.Fatalf("FileStatuses() error = %v", err)
	}
	if _, err := NewRunner(dir).DiscardOutput(ctx, statuses[0]); err != nil {
		t.Fatalf("DiscardOutput() error = %v", err)
	}
	got := strings.TrimSpace(runTestGit(t, dir, "status", "--porcelain", "-uall"))
	if got != "" {
		t.Fatalf("status after discard = %q, want clean", got)
	}
}

func TestFileStatusesCollapseUntrackedDirectories(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "existing.go"), []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("write src/existing.go: %v", err)
	}
	runTestGit(t, dir, "add", "src/existing.go")
	runTestGit(t, dir, "commit", "-qm", "add src")
	if err := os.MkdirAll(filepath.Join(dir, "src", "newdir"), 0o755); err != nil {
		t.Fatalf("mkdir src/newdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "newdir", "a.go"), []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write src/newdir/a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "new.go"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write src/new.go: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets", "icons"), 0o755); err != nil {
		t.Fatalf("mkdir assets/icons: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "icons", "add.svg"), []byte("svg\n"), 0o644); err != nil {
		t.Fatalf("write assets/icons/add.svg: %v", err)
	}

	statuses, err := NewRunner(dir).FileStatuses(ctx)
	if err != nil {
		t.Fatalf("FileStatuses() error = %v", err)
	}

	byPath := map[string]FileStatus{}
	for _, status := range statuses {
		byPath[status.Path] = status
	}
	if status, ok := byPath["src/newdir"]; !ok || !status.Directory {
		t.Fatalf("src/newdir status = %#v, ok %v, want directory row", status, ok)
	}
	if status, ok := byPath["assets"]; !ok || !status.Directory {
		t.Fatalf("assets status = %#v, ok %v, want directory row", status, ok)
	}
	if status, ok := byPath["src/new.go"]; !ok || status.Directory {
		t.Fatalf("src/new.go status = %#v, ok %v, want file row", status, ok)
	}
	if _, ok := byPath["src/newdir/a.go"]; ok {
		t.Fatalf("statuses include nested file under collapsed dir: %#v", statuses)
	}
}

func TestDiscardOutputRemovesStagedNewFile(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write new file: %v", err)
	}
	runTestGit(t, dir, "add", "new.txt")

	statuses, err := NewRunner(dir).FileStatuses(ctx)
	if err != nil {
		t.Fatalf("FileStatuses() error = %v", err)
	}
	if _, err := NewRunner(dir).DiscardOutput(ctx, statuses[0]); err != nil {
		t.Fatalf("DiscardOutput() error = %v", err)
	}
	got := strings.TrimSpace(runTestGit(t, dir, "status", "--porcelain", "-uall"))
	if got != "" {
		t.Fatalf("status after discard = %q, want clean", got)
	}
}

func TestDiscardOutputRestoresStagedRename(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	runTestGit(t, dir, "mv", "file.txt", "renamed.txt")

	statuses, err := NewRunner(dir).FileStatuses(ctx)
	if err != nil {
		t.Fatalf("FileStatuses() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].OldPath != "file.txt" || statuses[0].Path != "renamed.txt" {
		t.Fatalf("rename status = %#v, want file.txt -> renamed.txt", statuses)
	}
	if _, err := NewRunner(dir).DiscardOutput(ctx, statuses[0]); err != nil {
		t.Fatalf("DiscardOutput() error = %v", err)
	}
	got := strings.TrimSpace(runTestGit(t, dir, "status", "--porcelain", "-uall"))
	if got != "" {
		t.Fatalf("status after discard = %q, want clean", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "file.txt")); err != nil {
		t.Fatalf("file.txt should be restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "renamed.txt")); !os.IsNotExist(err) {
		t.Fatalf("renamed.txt should be removed, stat err = %v", err)
	}
}

// TestPushRejectedNonFastForward checks the detector that decides whether to
// offer a force push, across common git rejection phrasings.
func TestPushRejectedNonFastForward(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"non-fast-forward", errors.New("git push: ! [rejected] main -> main (non-fast-forward)"), true},
		{"updates rejected", errors.New("Updates were rejected because the tip of your current branch is behind"), true},
		{"failed to push", errors.New("error: failed to push some refs to 'origin'"), true},
		{"unrelated error", errors.New("fatal: Authentication failed"), false},
	}
	for _, tc := range cases {
		if got := PushRejectedNonFastForward(tc.err); got != tc.want {
			t.Errorf("PushRejectedNonFastForward(%q) = %t, want %t", tc.err, got, tc.want)
		}
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runTestGit(t, "", "init", "-q", dir)
	runTestGit(t, dir, "config", "user.name", "Test User")
	runTestGit(t, dir, "config", "user.email", "test@example.com")
	runTestGit(t, dir, "branch", "-M", "main")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runTestGit(t, dir, "add", "file.txt")
	runTestGit(t, dir, "commit", "-qm", "base")
	return dir
}

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runTestGitAllowError(t, dir, args...)
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func runTestGitAllowError(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
