package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSquashCandidatesListsCommitsAheadOfBase checks the squash picker only
// offers commits made on the feature branch, newest first.
func TestSquashCandidatesListsCommitsAheadOfBase(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	runTestGit(t, dir, "switch", "-q", "-c", "feature")
	writeCommit(t, dir, "a.txt", "one", "first feature change")
	writeCommit(t, dir, "b.txt", "two", "second feature change")

	commits, err := NewRunner(dir).SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() error = %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("len(commits) = %d, want 2", len(commits))
	}
	if commits[0].Subject != "second feature change" {
		t.Fatalf("commits[0].Subject = %q, want newest first", commits[0].Subject)
	}
	if commits[1].Subject != "first feature change" {
		t.Fatalf("commits[1].Subject = %q, want oldest last", commits[1].Subject)
	}
}

// TestSquashCandidatesRejectsDefaultBranch checks squashing is refused on the
// default branch instead of returning a confusing list.
func TestSquashCandidatesRejectsDefaultBranch(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)

	_, err := NewRunner(dir).SquashCandidates(ctx, "main")
	if err == nil {
		t.Fatal("SquashCandidates() error = nil, want default-branch refusal")
	}
	if !strings.Contains(err.Error(), "default branch") {
		t.Fatalf("SquashCandidates() error = %v, want default branch message", err)
	}
}

// TestResolveSquashPlan checks selections resolve to the right pick/squash
// actions with an explicit base commit.
func TestResolveSquashPlan(t *testing.T) {
	// newest-first: c3, c2, c1 (c1 is oldest).
	cases := []struct {
		name string
		sel  []bool
		base int
		want []string
	}{
		{"explicit newest base", []bool{false, true, true}, 0, []string{"pick", "squash", "squash"}},
		{"middle base", []bool{true, false, true}, 1, []string{"squash", "pick", "squash"}},
		{"oldest base", []bool{true, true, false}, 2, []string{"squash", "squash", "pick"}},
		{"base wins over squash", []bool{true, true, true}, 1, []string{"squash", "pick", "squash"}},
		{"no base keeps all", []bool{false, true, true}, -1, []string{"pick", "pick", "pick"}},
	}
	for _, tc := range cases {
		actions := []SquashAction{
			{Hash: "c3", Squash: tc.sel[0]},
			{Hash: "c2", Squash: tc.sel[1]},
			{Hash: "c1", Squash: tc.sel[2]},
		}
		if tc.base >= 0 {
			actions[tc.base].Base = true
		}
		plan := ResolveSquashPlan(actions)
		for i, line := range plan {
			if line.Action != tc.want[i] {
				t.Errorf("%s: plan[%d].Action = %q, want %q", tc.name, i, line.Action, tc.want[i])
			}
		}
	}
}

// TestApplySquashFoldsAllCommits checks selecting every commit (oldest included)
// collapses the branch into a single commit without losing any work.
func TestApplySquashFoldsAllCommits(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	runTestGit(t, dir, "switch", "-q", "-c", "feature")
	writeCommit(t, dir, "a.txt", "one", "first feature change")
	writeCommit(t, dir, "b.txt", "two", "second feature change")
	writeCommit(t, dir, "c.txt", "three", "third feature change")

	runner := NewRunner(dir)
	commits, err := runner.SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() error = %v", err)
	}

	// Use the default newest commit as base; squash the older commits into it.
	actions := []SquashAction{
		{Hash: commits[0].Hash, Base: true},
		{Hash: commits[1].Hash, Squash: true},
		{Hash: commits[2].Hash, Squash: true},
	}
	if _, err := runner.ApplySquash(ctx, "main", actions); err != nil {
		t.Fatalf("ApplySquash() error = %v", err)
	}

	after, err := runner.SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() after squash error = %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("len(commits) after squash = %d, want 1", len(after))
	}
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist after squash: %v", name, err)
		}
	}
}

// TestApplySquashFoldsDependentCommitsIntoNewestBase checks the default
// newest-base flow keeps chronological patch order, so normal dependent changes
// to the same file do not conflict.
func TestApplySquashFoldsDependentCommitsIntoNewestBase(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	runTestGit(t, dir, "switch", "-q", "-c", "feature")
	writeCommit(t, dir, "a.txt", "one", "first")
	writeCommit(t, dir, "a.txt", "one\ntwo", "second")
	writeCommit(t, dir, "a.txt", "one\ntwo\nthree", "third")

	runner := NewRunner(dir)
	commits, err := runner.SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() error = %v", err)
	}

	actions := []SquashAction{
		{Hash: commits[0].Hash, Base: true},
		{Hash: commits[1].Hash, Squash: true},
		{Hash: commits[2].Hash, Squash: true},
	}
	if _, err := runner.ApplySquash(ctx, "main", actions); err != nil {
		t.Fatalf("ApplySquash() error = %v", err)
	}

	after, err := runner.SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() after squash error = %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("len(commits) after squash = %d, want 1", len(after))
	}
	content, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatalf("read a.txt: %v", err)
	}
	if string(content) != "one\ntwo\nthree\n" {
		t.Fatalf("a.txt = %q, want final content", content)
	}
}

// TestApplySquashCanMoveBase checks the user can choose a base other than the
// newest commit.
func TestApplySquashCanMoveBase(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	runTestGit(t, dir, "switch", "-q", "-c", "feature")
	writeCommit(t, dir, "a.txt", "one", "first")
	writeCommit(t, dir, "b.txt", "two", "second")
	writeCommit(t, dir, "c.txt", "three", "third")

	runner := NewRunner(dir)
	commits, err := runner.SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() error = %v", err)
	}

	// Use the oldest commit as the base, squash the middle commit into it, and
	// keep the newest commit alone.
	actions := []SquashAction{
		{Hash: commits[0].Hash, Squash: false},
		{Hash: commits[1].Hash, Squash: true},
		{Hash: commits[2].Hash, Base: true},
	}
	if _, err := runner.ApplySquash(ctx, "main", actions); err != nil {
		t.Fatalf("ApplySquash() error = %v", err)
	}

	after, err := runner.SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() after squash error = %v", err)
	}
	if len(after) != 2 {
		t.Fatalf("len(commits) after squash = %d, want 2", len(after))
	}
}

// TestApplySquashIgnoresAdvancedBase reproduces the "odd behaviour with base"
// case: the default branch gains new commits after this branch forked. Squashing
// must collapse only this branch's commits, without replaying them onto the new
// base tip or pulling the base's new commits into the branch.
func TestApplySquashIgnoresAdvancedBase(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)

	// Branch off main, add two commits.
	runTestGit(t, dir, "switch", "-q", "-c", "feature")
	writeCommit(t, dir, "a.txt", "one", "first")
	writeCommit(t, dir, "b.txt", "two", "second")

	// main moves on independently after the branch point.
	runTestGit(t, dir, "switch", "-q", "main")
	writeCommit(t, dir, "main-only.txt", "main work", "main moves on")
	runTestGit(t, dir, "switch", "-q", "feature")

	runner := NewRunner(dir)
	commits, err := runner.SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() error = %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("candidates = %d, want 2 (only this branch's commits)", len(commits))
	}

	actions := []SquashAction{
		{Hash: commits[0].Hash, Base: true},
		{Hash: commits[1].Hash, Squash: true},
	}
	if _, err := runner.ApplySquash(ctx, "main", actions); err != nil {
		t.Fatalf("ApplySquash() error = %v", err)
	}

	after, err := runner.SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() after squash error = %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("commits after squash = %d, want 1", len(after))
	}
	// The branch must NOT have been replayed onto main's new tip: main's commit
	// stays absent from the feature branch.
	if _, err := os.Stat(filepath.Join(dir, "main-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("main-only.txt present on feature branch — branch was rebased onto the new base tip")
	}
}

// TestApplySquashRejectsNoSquashSelection checks selecting only a base is
// refused instead of a no-op squash.
func TestApplySquashRejectsNoSquashSelection(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	runTestGit(t, dir, "switch", "-q", "-c", "feature")
	writeCommit(t, dir, "a.txt", "one", "first")
	writeCommit(t, dir, "b.txt", "two", "second")

	runner := NewRunner(dir)
	commits, err := runner.SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() error = %v", err)
	}
	actions := []SquashAction{
		{Hash: commits[0].Hash, Base: true},
		{Hash: commits[1].Hash},
	}
	if _, err := runner.ApplySquash(ctx, "main", actions); err == nil {
		t.Fatal("ApplySquash() with no squash selection error = nil, want refusal")
	}
}

// TestApplySquashRejectsNoBaseSelection checks selecting commits without a
// base is refused.
func TestApplySquashRejectsNoBaseSelection(t *testing.T) {
	ctx := context.Background()
	dir := initTestRepo(t)
	runTestGit(t, dir, "switch", "-q", "-c", "feature")
	writeCommit(t, dir, "a.txt", "one", "first")
	writeCommit(t, dir, "b.txt", "two", "second")

	runner := NewRunner(dir)
	commits, err := runner.SquashCandidates(ctx, "main")
	if err != nil {
		t.Fatalf("SquashCandidates() error = %v", err)
	}
	actions := []SquashAction{
		{Hash: commits[0].Hash, Squash: true},
		{Hash: commits[1].Hash, Squash: true},
	}
	if _, err := runner.ApplySquash(ctx, "main", actions); err == nil {
		t.Fatal("ApplySquash() without a base error = nil, want refusal")
	}
}

func writeCommit(t *testing.T, dir, name, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	runTestGit(t, dir, "add", name)
	runTestGit(t, dir, "commit", "-qm", message)
}
