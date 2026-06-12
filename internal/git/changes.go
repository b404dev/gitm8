package git

import (
	"context"
	"strings"
)

// Stage

// Stage stages the provided paths without returning command output.
func (r Runner) Stage(ctx context.Context, paths ...string) error {
	args := append([]string{"add", "--"}, paths...)
	return r.run(ctx, args...)
}

// StageOutput stages the provided paths and returns Git's normal output for the UI.
func (r Runner) StageOutput(ctx context.Context, paths ...string) (string, error) {
	args := append([]string{"add", "--"}, paths...)
	return r.output(ctx, args...)
}

// StageAllOutput stages every changed and untracked file in the repo.
func (r Runner) StageAllOutput(ctx context.Context) (string, error) {
	return r.output(ctx, "add", "--all")
}

// Unstage

// Unstage removes the provided paths from the index without returning output.
func (r Runner) Unstage(ctx context.Context, paths ...string) error {
	args := append([]string{"restore", "--staged", "--"}, paths...)
	return r.run(ctx, args...)
}

// UnstageOutput removes paths from the index and returns Git's normal output.
func (r Runner) UnstageOutput(ctx context.Context, paths ...string) (string, error) {
	args := append([]string{"restore", "--staged", "--"}, paths...)
	return r.output(ctx, args...)
}

// UnstageAllOutput removes every staged path from the index.
func (r Runner) UnstageAllOutput(ctx context.Context) (string, error) {
	return r.output(ctx, "restore", "--staged", "--", ".")
}

// Diff And Review

// Diff returns the unstaged diff, optionally scoped to a single path.
func (r Runner) Diff(ctx context.Context, paths ...string) (string, error) {
	args := []string{"diff"}
	if len(paths) > 0 {
		args = append(args, "--", paths[0])
	}
	return r.output(ctx, args...)
}

// StagedDiff returns the staged diff, optionally limited to one path.
func (r Runner) StagedDiff(ctx context.Context, paths ...string) (string, error) {
	args := []string{"diff", "--cached"}
	if len(paths) > 0 {
		args = append(args, "--", paths[0])
	}
	return r.output(ctx, args...)
}

// Review combines staged and unstaged diffs into one text block for review.
func (r Runner) Review(ctx context.Context, paths ...string) (string, error) {
	staged, err := r.StagedDiff(ctx, paths...)
	if err != nil {
		return "", err
	}

	unstaged, err := r.Diff(ctx, paths...)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	if strings.TrimSpace(staged) != "" {
		b.WriteString("STAGED\n")
		b.WriteString(staged)
		if !strings.HasSuffix(staged, "\n") {
			b.WriteString("\n")
		}
	}
	if strings.TrimSpace(unstaged) != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("UNSTAGED\n")
		b.WriteString(unstaged)
	}
	if b.Len() == 0 {
		return "No diff to review.\n", nil
	}
	return b.String(), nil
}
