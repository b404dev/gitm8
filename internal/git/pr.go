package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PullRequestOutput uses GitHub CLI to show an open PR or create one for the branch.
func (r Runner) PullRequestOutput(ctx context.Context) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh CLI is required for pull requests: install gh and run gh auth login")
	}
	if _, err := r.commandOutput(ctx, "gh", "auth", "token", "-h", "github.com"); err != nil {
		return "", fmt.Errorf("GitHub CLI token not found; run gh auth login -h github.com")
	}

	branch := strings.TrimSpace(r.bestEffort(ctx, "branch", "--show-current"))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("pull requests require a named branch")
	}

	// Prefer showing an existing open PR for this branch to avoid duplicates.
	if out, err := r.commandOutput(ctx, "gh", "pr", "view", "--json", "url,state", "--jq", `select(.state == "OPEN") | .url`); err == nil {
		url := strings.TrimSpace(out)
		if url != "" {
			return "Pull request: " + url + "\n", nil
		}
	}

	var outputs []string
	if r.bestEffort(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}") == "" {
		pushed, err := r.output(ctx, "push", "-u", "origin", branch)
		if err != nil {
			return pushed, err
		}
		outputs = append(outputs, pushed)
	}

	out, err := r.commandOutput(ctx, "gh", "pr", "create", "--fill")
	if err != nil {
		return joinOutput(append(outputs, out)...), err
	}
	if strings.TrimSpace(out) == "" {
		outputs = append(outputs, "Pull request created.")
		return joinOutput(outputs...) + "\n", nil
	}

	outputs = append(outputs, out)
	out = joinOutput(outputs...)
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}
