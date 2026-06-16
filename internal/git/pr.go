package git

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/b404dev/gitm8/internal/logging"
)

const maxPullRequestDiffBytes = 120000

// PullRequestOutput uses GitHub CLI to show an open PR or create one for the branch.
func (r Runner) PullRequestOutput(ctx context.Context, baseBranch string, provider string, ollamaURL string) (string, error) {
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = "main"
	}
	logging.Info("git", "PullRequestOutput", "pr_create_start", logging.F("base", baseBranch), logging.F("provider", provider))
	if _, err := exec.LookPath("gh"); err != nil {
		logging.Error("git", "PullRequestOutput", "gh_missing", logging.F("error", err))
		return "", fmt.Errorf("gh CLI is required for pull requests: install gh and run gh auth login")
	}
	if _, err := r.commandOutput(ctx, "gh", "auth", "token", "-h", "github.com"); err != nil {
		logging.Error("git", "PullRequestOutput", "gh_auth_missing", logging.F("error", err))
		return "", fmt.Errorf("GitHub CLI token not found; run gh auth login -h github.com")
	}

	branch := strings.TrimSpace(r.bestEffort(ctx, "branch", "--show-current"))
	if branch == "" || branch == "HEAD" {
		logging.Warn("git", "PullRequestOutput", "unnamed_branch")
		return "", fmt.Errorf("pull requests require a named branch")
	}
	logging.Info("git", "PullRequestOutput", "branch_detected", logging.F("branch", branch))

	// Prefer showing an existing open PR for this branch to avoid duplicates.
	if out, err := r.commandOutput(ctx, "gh", "pr", "view", "--json", "url,state", "--jq", `select(.state == "OPEN") | .url`); err == nil {
		url := strings.TrimSpace(out)
		if url != "" {
			logging.Info("git", "PullRequestOutput", "existing_pr_found", logging.F("url", url))
			return "Pull request: " + url + "\n", nil
		}
	}

	if r.bestEffort(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}") == "" {
		logging.Warn("git", "PullRequestOutput", "branch_not_pushed", logging.F("branch", branch))
		return "", fmt.Errorf("current branch is not pushed; push it first, then create the PR")
	}

	title, body, err := r.GeneratePullRequestDraft(ctx, baseBranch, provider, ollamaURL)
	if err != nil {
		logging.Error("git", "PullRequestOutput", "pr_draft_failed", logging.F("error", err))
		return "", err
	}

	out, err := r.commandOutput(ctx, "gh", "pr", "create", "--base", baseBranch, "--title", title, "--body", body)
	if err != nil {
		logging.Error("git", "PullRequestOutput", "pr_create_failed", logging.F("error", err))
		return out, err
	}
	logging.Info("git", "PullRequestOutput", "pr_create_complete", logging.F("title_bytes", len(title)), logging.F("body_bytes", len(body)))
	if strings.TrimSpace(out) == "" {
		return "Pull request created.\n", nil
	}

	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}

// GeneratePullRequestDraft asks the configured AI CLI for a PR title and body
// based on everything this branch changes versus baseBranch.
func (r Runner) GeneratePullRequestDraft(ctx context.Context, baseBranch string, provider string, ollamaURL string) (string, string, error) {
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = "main"
	}
	baseRef := r.pullRequestBaseRef(ctx, baseBranch)
	logging.Info("git", "GeneratePullRequestDraft", "pr_draft_generate_start", logging.F("base", baseBranch), logging.F("base_ref", baseRef), logging.F("provider", provider))

	diff, err := r.output(ctx, "diff", "--stat", baseRef+"...HEAD")
	if err != nil {
		logging.Error("git", "GeneratePullRequestDraft", "diff_stat_failed", logging.F("base_ref", baseRef), logging.F("error", err))
		return "", "", err
	}
	fullDiff, err := r.output(ctx, "diff", baseRef+"...HEAD")
	if err != nil {
		logging.Error("git", "GeneratePullRequestDraft", "diff_failed", logging.F("base_ref", baseRef), logging.F("error", err))
		return "", "", err
	}
	if strings.TrimSpace(fullDiff) == "" {
		logging.Warn("git", "GeneratePullRequestDraft", "empty_branch_diff", logging.F("base_ref", baseRef))
		return "", "", fmt.Errorf("no branch changes found versus %s", baseRef)
	}
	if len(fullDiff) > maxPullRequestDiffBytes {
		logging.Warn("git", "GeneratePullRequestDraft", "diff_too_large", logging.F("diff_bytes", len(fullDiff)), logging.F("limit_bytes", maxPullRequestDiffBytes))
		return "", "", fmt.Errorf("branch diff is too large for AI PR generation; write the PR manually")
	}

	commits := strings.TrimSpace(r.bestEffort(ctx, "log", "--oneline", baseRef+"..HEAD"))
	prompt := `You are the developer who made the code changes on this branch.
Write a pull request title and description for everything changed versus the base branch.

Guidelines:
- Describe the user-visible or developer-visible change, not the file list.
- Keep the title concise and practical.
- Use a short markdown body with a Summary section.
- Do not use em dashes.
- Return only valid JSON with this shape:
{"title":"one-line title","body":"markdown body"}

BASE BRANCH:
` + baseBranch + `

COMMITS:
` + commits + `

DIFF STAT:
` + diff + `

FULL DIFF:
` + fullDiff

	out, err := r.generateAIText(ctx, provider, ollamaURL, prompt)
	if err != nil {
		logging.Error("git", "GeneratePullRequestDraft", "pr_draft_generate_failed",
			logging.F("provider", provider),
			logging.F("diff_bytes", len(fullDiff)),
			logging.F("prompt_bytes", len(prompt)),
			logging.F("error", err),
		)
		return "", "", err
	}
	title, body := parsePullRequestDraft(out)
	if title == "" || body == "" {
		logging.Error("git", "GeneratePullRequestDraft", "pr_draft_invalid", logging.F("output_bytes", len(out)))
		return "", "", fmt.Errorf("AI returned an invalid pull request draft")
	}
	logging.Info("git", "GeneratePullRequestDraft", "pr_draft_generate_complete",
		logging.F("provider", provider),
		logging.F("diff_bytes", len(fullDiff)),
		logging.F("prompt_bytes", len(prompt)),
		logging.F("title_bytes", len(title)),
		logging.F("body_bytes", len(body)),
	)
	return title, body, nil
}

// ManualPullRequestCommand builds the interactive gh command used when the user
// wants to write the title and body themselves.
func (r Runner) ManualPullRequestCommand(baseBranch string) (*exec.Cmd, error) {
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = "main"
	}
	logging.Info("git", "ManualPullRequestCommand", "manual_pr_command_create", logging.F("base", baseBranch))
	path, err := exec.LookPath("gh")
	if err != nil {
		logging.Error("git", "ManualPullRequestCommand", "gh_missing", logging.F("error", err))
		return nil, fmt.Errorf("gh CLI is required for pull requests")
	}
	cmd := exec.Command(path, "pr", "create", "--base", baseBranch)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	return cmd, nil
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
	out = strings.TrimPrefix(out, "```json")
	out = strings.TrimPrefix(out, "```")
	out = strings.TrimSuffix(out, "```")
	out = strings.TrimSpace(out)

	var draft struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.Unmarshal([]byte(out), &draft); err != nil {
		return "", ""
	}
	return strings.TrimSpace(draft.Title), strings.TrimSpace(draft.Body)
}
