package git

import (
	"bufio"
	"context"
	"fmt"
	"os"
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

// ConflictFiles returns files Git currently considers unmerged.
func (r Runner) ConflictFiles(ctx context.Context) ([]string, error) {
	out, err := r.output(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(out), nil
}

// MarkResolvedOutput stages a conflict file after the user resolves it.
func (r Runner) MarkResolvedOutput(ctx context.Context, path string) (string, error) {
	return r.StageOutput(ctx, path)
}

// ConflictMarkerReport summarizes conflict marker lines in a working tree file.
func (r Runner) ConflictMarkerReport(path string) string {
	file, err := os.Open(r.workingPath(path))
	if err != nil {
		return "Conflict markers: unable to read file.\n"
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if isConflictMarker(line) {
			lines = append(lines, fmt.Sprintf("%d: %s", lineNo, line))
			if len(lines) >= 30 {
				lines = append(lines, "...")
				break
			}
		}
	}
	if len(lines) == 0 {
		return "Conflict markers: none found in selected file.\n"
	}
	return "Conflict markers:\n" + strings.Join(lines, "\n") + "\n"
}

func isConflictMarker(line string) bool {
	return strings.HasPrefix(line, "<<<<<<<") ||
		strings.HasPrefix(line, "=======") ||
		strings.HasPrefix(line, ">>>>>>>")
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

// DiscardOutput removes all working tree and index changes for one status row.
func (r Runner) DiscardOutput(ctx context.Context, file FileStatus) (string, error) {
	path := file.Path
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	if file.Index == '?' {
		return r.output(ctx, "clean", "-fd", "--", path)
	}

	var outputs []string
	if file.Renamed() {
		args := append([]string{"restore", "--staged", "--"}, file.GitPaths()...)
		out, err := r.output(ctx, args...)
		outputs = append(outputs, out)
		if err != nil {
			return joinOutput(outputs...), err
		}

		clean, cleanErr := r.output(ctx, "clean", "-fd", "--", file.Path)
		outputs = append(outputs, clean)
		if cleanErr != nil {
			return joinOutput(outputs...), cleanErr
		}

		restore, restoreErr := r.output(ctx, "restore", "--worktree", "--", file.OldPath)
		outputs = append(outputs, restore)
		if restoreErr != nil {
			return joinOutput(outputs...), restoreErr
		}
		return joinOutput(outputs...), nil
	}

	if file.Staged() {
		out, err := r.output(ctx, "restore", "--staged", "--", path)
		outputs = append(outputs, out)
		if err != nil {
			return joinOutput(outputs...), err
		}
	}

	if file.Index == 'A' {
		out, err := r.output(ctx, "clean", "-fd", "--", path)
		outputs = append(outputs, out)
		if err != nil {
			return joinOutput(outputs...), err
		}
		return joinOutput(outputs...), nil
	}

	out, err := r.output(ctx, "restore", "--worktree", "--", path)
	outputs = append(outputs, out)
	if err != nil {
		return joinOutput(outputs...), err
	}
	return joinOutput(outputs...), nil
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
