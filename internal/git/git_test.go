package git

import (
	"reflect"
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

// TestParseStatusLine checks parsing of `git status --porcelain` lines.
func TestParseStatusLine(t *testing.T) {
	cases := []struct {
		name         string
		line         string
		wantPath     string
		wantIndex    byte
		wantWorktree byte
		wantOK       bool
	}{
		{"worktree only", " M file.txt", "file.txt", ' ', 'M', true},
		{"staged", "M  file.txt", "file.txt", 'M', ' ', true},
		{"staged and worktree", "MM file.txt", "file.txt", 'M', 'M', true},
		{"untracked", "?? new.txt", "new.txt", '?', '?', true},
		{"rename", "R  old.txt -> new.txt", "new.txt", 'R', ' ', true},
		{"path with spaces", " M my file.txt", "my file.txt", ' ', 'M', true},
		{"too short", " M", "", 0, 0, false},
		{"blank", "", "", 0, 0, false},
	}
	for _, tc := range cases {
		got, ok := parseStatusLine(tc.line)
		if ok != tc.wantOK {
			t.Errorf("parseStatusLine(%q) ok = %t, want %t", tc.line, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if got.Path != tc.wantPath || got.Index != tc.wantIndex || got.Worktree != tc.wantWorktree {
			t.Errorf("parseStatusLine(%q) = {Path:%q Index:%q Worktree:%q}, want {Path:%q Index:%q Worktree:%q}",
				tc.line, got.Path, got.Index, got.Worktree, tc.wantPath, tc.wantIndex, tc.wantWorktree)
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
