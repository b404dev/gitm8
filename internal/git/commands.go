package git

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Status And Sync Commands

// Status returns the same compact status as `git status --short --branch`.
func (r Runner) Status(ctx context.Context) (string, error) {
	return r.output(ctx, "status", "--short", "--branch")
}

// Fetch updates all remotes and removes stale remote branch references.
func (r Runner) Fetch(ctx context.Context) error {
	return r.run(ctx, "fetch", "--all", "--prune")
}

// FetchOutput fetches remotes and returns Git's normal output for display.
func (r Runner) FetchOutput(ctx context.Context) (string, error) {
	return r.output(ctx, "fetch", "--all", "--prune")
}

// Pull runs `git pull --ff-only` without returning output.
func (r Runner) Pull(ctx context.Context) error {
	return r.run(ctx, "pull", "--ff-only")
}

// PullOutput runs `git pull --ff-only` and returns Git's normal output.
func (r Runner) PullOutput(ctx context.Context) (string, error) {
	return r.output(ctx, "pull", "--ff-only")
}

// Push pushes the current branch to its configured remote branch.
func (r Runner) Push(ctx context.Context) error {
	return r.run(ctx, "push")
}

// PushOutput pushes the current branch and appends the remote web URL if known.
func (r Runner) PushOutput(ctx context.Context) (string, error) {
	return r.pushOutputWithRepoURL(ctx)
}

// Git Config Commands

// SetUpstreamOutput sets which remote branch the current local branch tracks.
func (r Runner) SetUpstreamOutput(ctx context.Context, remote string, branch string) (string, error) {
	remoteOut, err := r.output(ctx, "config", "branch."+branch+".remote", remote)
	if err != nil {
		return remoteOut, err
	}

	mergeOut, err := r.output(ctx, "config", "branch."+branch+".merge", "refs/heads/"+branch)
	if err != nil {
		return joinOutput(remoteOut, mergeOut), err
	}
	return joinOutput(remoteOut, mergeOut, "Set upstream config to "+remote+"/"+branch), nil
}

// SetUserOutput sets the Git identity for this repo only. It does not change
// the user's global Git config.
func (r Runner) SetUserOutput(ctx context.Context, name string, email string) (string, error) {
	nameOut, err := r.output(ctx, "config", "user.name", name)
	if err != nil {
		return nameOut, err
	}

	emailOut, err := r.output(ctx, "config", "user.email", email)
	if err != nil {
		return joinOutput(nameOut, emailOut), err
	}
	return joinOutput(nameOut, emailOut, "Set identity to "+name+" <"+email+">"), nil
}

// Commit Commands

// Commit creates a commit with the provided message and suppresses output.
func (r Runner) Commit(ctx context.Context, message string) error {
	return r.run(ctx, "commit", "-m", message)
}

// CommitOutput creates a commit and returns Git's normal output for the output panel.
func (r Runner) CommitOutput(ctx context.Context, message string) (string, error) {
	return r.output(ctx, "commit", "-m", message)
}

// GenerateCommitMessage asks the configured AI CLI for one commit subject based
// on staged changes.
func (r Runner) GenerateCommitMessage(ctx context.Context, provider string) (string, error) {
	diff, err := r.StagedDiff(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(diff) == "" {
		return "", fmt.Errorf("no staged changes to describe")
	}

	prompt := `You are the developer who made the staged code changes below.
Write the Git commit subject you would use for this commit.

Guidelines:
- Describe what changed, not that files changed.
- Prefer a concise, practical developer voice.
- Use imperative mood, like "Add", "Fix", "Update", or "Remove".
- Keep it under 72 characters.
- Return only one subject line.
- Do not include quotes, markdown, bullets, explanations, or alternatives.

STAGED DIFF:
` + diff

	message, err := r.generateAIText(ctx, provider, prompt)
	if err != nil {
		return "", err
	}
	return cleanCommitSubject(message), nil
}

func (r Runner) generateAIText(ctx context.Context, provider string, prompt string) (string, error) {
	if strings.ToLower(strings.TrimSpace(provider)) == "claude" {
		return r.commandInputOutput(ctx, prompt, "claude",
			"-p",
			"--permission-mode", "dontAsk",
			"--output-format", "text",
			"--no-session-persistence",
		)
	}
	return r.generateCodexText(ctx, prompt)
}

func (r Runner) generateCodexText(ctx context.Context, prompt string) (string, error) {
	outFile, err := os.CreateTemp("", "gitm8-commit-message-*")
	if err != nil {
		return "", err
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	_, err = r.commandInputOutput(ctx, prompt, "codex", "exec",
		"--sandbox", "read-only",
		"--color", "never",
		"--ephemeral",
		"-o", outPath,
		"-",
	)
	if err != nil {
		return "", err
	}

	message, err := os.ReadFile(outPath)
	if err != nil {
		return "", err
	}
	return string(message), nil
}

func cleanCommitSubject(message string) string {
	message = strings.TrimSpace(message)
	message = strings.Trim(message, "`\"'")
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.Trim(line, "`\"'"))
		if line != "" {
			return line
		}
	}
	return ""
}

// Push Output Helpers

// pushOutputWithRepoURL wraps push output with a browsable repository URL.
func (r Runner) pushOutputWithRepoURL(ctx context.Context) (string, error) {
	out, err := r.output(ctx, "push")
	url := r.repoWebURL(ctx)
	if url == "" {
		return out, err
	}
	if err != nil {
		return out, fmt.Errorf("%w\nPushed to %s", err, url)
	}
	return joinOutput(out, "Pushed to "+url) + "\n", nil
}
