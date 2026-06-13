package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PullRequestOutput uses GitHub CLI to show an open PR or create one for the branch.
func (r Runner) PullRequestOutput(ctx context.Context, baseBranch string, provider string) (string, error) {
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = "main"
	}
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

	title, body, err := r.GeneratePullRequestDraft(ctx, baseBranch, provider)
	if err != nil {
		return joinOutput(outputs...), err
	}

	out, err := r.commandOutput(ctx, "gh", "pr", "create", "--base", baseBranch, "--title", title, "--body", body)
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

// GeneratePullRequestDraft asks the configured AI CLI for a PR title and body
// based on everything this branch changes versus baseBranch.
func (r Runner) GeneratePullRequestDraft(ctx context.Context, baseBranch string, provider string) (string, string, error) {
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = "main"
	}
	baseRef := r.pullRequestBaseRef(ctx, baseBranch)

	diff, err := r.output(ctx, "diff", "--stat", baseRef+"...HEAD")
	if err != nil {
		return "", "", err
	}
	fullDiff, err := r.output(ctx, "diff", baseRef+"...HEAD")
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(fullDiff) == "" {
		return "", "", fmt.Errorf("no branch changes found versus %s", baseRef)
	}

	commits := strings.TrimSpace(r.bestEffort(ctx, "log", "--oneline", baseRef+"..HEAD"))
	prompt := `You are the developer who made the code changes on this branch.
Write a pull request title and description for everything changed versus the base branch.

Guidelines:
- Describe the user-visible or developer-visible change, not the file list.
- Keep the title concise and practical.
- Use a short markdown body with a Summary section and a Testing section.
- If testing is not shown in the diff, say "Not run".
- Return exactly this format:
TITLE: <one-line title>
BODY:
<markdown body>

BASE BRANCH:
` + baseBranch + `

COMMITS:
` + commits + `

DIFF STAT:
` + diff + `

FULL DIFF:
` + fullDiff

	out, err := r.generateAIText(ctx, provider, prompt)
	if err != nil {
		return "", "", err
	}
	title, body := parsePullRequestDraft(out)
	if title == "" || body == "" {
		return "", "", fmt.Errorf("AI returned an invalid pull request draft")
	}
	return title, body, nil
}

func (r Runner) pullRequestBaseRef(ctx context.Context, baseBranch string) string {
	if strings.TrimSpace(r.bestEffort(ctx, "rev-parse", "--verify", baseBranch)) != "" {
		return baseBranch
	}
	remote := r.defaultRemote(ctx)
	if remote != "" && strings.TrimSpace(r.bestEffort(ctx, "rev-parse", "--verify", remote+"/"+baseBranch)) != "" {
		return remote + "/" + baseBranch
	}
	return baseBranch
}

func parsePullRequestDraft(out string) (string, string) {
	out = strings.TrimSpace(out)
	titlePrefix := "TITLE:"
	bodyPrefix := "BODY:"
	titleAt := strings.Index(out, titlePrefix)
	bodyAt := strings.Index(out, bodyPrefix)
	if titleAt < 0 || bodyAt < 0 || bodyAt < titleAt {
		return "", ""
	}

	title := strings.TrimSpace(out[titleAt+len(titlePrefix) : bodyAt])
	body := strings.TrimSpace(out[bodyAt+len(bodyPrefix):])
	title = strings.Trim(title, "`\"'")
	return title, body
}
