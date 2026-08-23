package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/b404dev/gitm8/internal/logging"
)

var ollamaHTTPClient = http.DefaultClient

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
	return r.pushOutputWithRepoURL(ctx, "")
}

// ForcePushOutput force-pushes the current branch with --force-with-lease, the
// safe force that refuses to clobber commits the remote gained since the last
// fetch. It is used to publish a branch after squashing rewrote its history.
func (r Runner) ForcePushOutput(ctx context.Context) (string, error) {
	return r.pushOutputWithRepoURL(ctx, "--force-with-lease")
}

// ForcePushUnconditionalOutput force-pushes the current branch with a raw
// --force, overwriting the remote regardless of what it gained since the last
// fetch. This has no safety net; it is used by matt_mode.
func (r Runner) ForcePushUnconditionalOutput(ctx context.Context) (string, error) {
	return r.pushOutputWithRepoURL(ctx, "--force")
}

// Git Config Commands

// GlobalConfigValue reads one optional value from the user's global Git config.
func (r Runner) GlobalConfigValue(ctx context.Context, key string) string {
	return strings.TrimSpace(r.bestEffort(ctx, "config", "--global", "--get", key))
}

// ConfigureGlobalIdentityOutput writes first-run choices to global Git config.
func (r Runner) ConfigureGlobalIdentityOutput(ctx context.Context, name, email, defaultBranch string) (string, error) {
	settings := [][2]string{{"user.name", name}, {"user.email", email}, {"init.defaultBranch", defaultBranch}}
	var outputs []string
	for _, setting := range settings {
		out, err := r.output(ctx, "config", "--global", setting[0], setting[1])
		outputs = append(outputs, out)
		if err != nil {
			return joinOutput(outputs...), err
		}
	}
	outputs = append(outputs, "Configured global Git identity and default branch.")
	return joinOutput(outputs...), nil
}

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
func (r Runner) GenerateCommitMessage(ctx context.Context, provider string, ollamaURL string) (string, error) {
	logging.Info("git", "GenerateCommitMessage", "commit_message_generate_start", logging.F("provider", provider))
	diff, err := r.StagedDiff(ctx)
	if err != nil {
		logging.Error("git", "GenerateCommitMessage", "staged_diff_failed", logging.F("error", err))
		return "", err
	}
	if strings.TrimSpace(diff) == "" {
		logging.Warn("git", "GenerateCommitMessage", "no_staged_changes")
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
- Do not use em dashes.
- Do not include quotes, markdown, bullets, explanations, or alternatives.

STAGED DIFF:
` + diff

	message, err := r.generateAIText(ctx, provider, ollamaURL, prompt)
	if err != nil {
		logging.Error("git", "GenerateCommitMessage", "commit_message_generate_failed",
			logging.F("provider", provider),
			logging.F("diff_bytes", len(diff)),
			logging.F("prompt_bytes", len(prompt)),
			logging.F("error", err),
		)
		return "", fmt.Errorf("failed to generate commit message using %s: %w", provider, err)
	}
	cleaned := cleanCommitSubject(message)
	logging.Info("git", "GenerateCommitMessage", "commit_message_generate_complete",
		logging.F("provider", provider),
		logging.F("diff_bytes", len(diff)),
		logging.F("prompt_bytes", len(prompt)),
		logging.F("message_bytes", len(cleaned)),
	)
	return cleaned, nil
}

// CommitWithAI generates a commit message using the configured AI provider and then commits it.
func (r Runner) CommitWithAI(ctx context.Context, provider string, ollamaURL string) (string, error) {
	message, err := r.GenerateCommitMessage(ctx, provider, ollamaURL)
	if err != nil {
		return "", fmt.Errorf("failed to generate commit message: %w", err)
	}

	logging.Info("git", "CommitWithAI", "commit_execution_start", logging.F("message", message))
	// Use CommitOutput to perform the commit and capture its output for display
	output, err := r.CommitOutput(ctx, message)
	if err != nil {
		return "", fmt.Errorf("failed to execute commit: %w", err)
	}

	logging.Info("git", "CommitWithAI", "commit_execution_complete")
	// Return the output from the successful commit operation
	return joinOutput(output, fmt.Sprintf("\nSuccessfully committed with message: %s", message)), nil
}

func (r Runner) generateAIText(ctx context.Context, provider string, ollamaURL string, prompt string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "claude" {
		logging.Info("git", "generateAIText", "ai_provider_selected", logging.F("provider", "claude"), logging.F("prompt_bytes", len(prompt)))
		return r.commandInputOutput(ctx, prompt, "claude",
			"-p",
			"--permission-mode", "dontAsk",
			"--output-format", "text",
			"--no-session-persistence",
		)
	}
	if provider == "ollama" {
		logging.Info("git", "generateAIText", "ai_provider_selected", logging.F("provider", "ollama"), logging.F("url", ollamaURL), logging.F("prompt_bytes", len(prompt)))
		return r.generateOllamaText(ctx, ollamaURL, prompt)
	}
	logging.Info("git", "generateAIText", "ai_provider_selected", logging.F("provider", "codex"), logging.F("prompt_bytes", len(prompt)))
	return r.generateCodexText(ctx, prompt)
}

func (r Runner) GenerateOllamaText(ctx context.Context, url string, prompt string) (string, error) {
	return r.generateOllamaText(ctx, url, prompt)
}

func (r Runner) generateOllamaText(ctx context.Context, url string, prompt string) (string, error) {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if url == "" {
		url = "http://localhost:11434"
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	model, err := resolveOllamaModel(ctx, url)
	if err != nil {
		return "", fmt.Errorf("failed to resolve Ollama model: %w", err)
	}
	body, err := json.Marshal(map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create Ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ollamaHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed (check service status): %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Ollama response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama API error (%s): %s", resp.Status, strings.TrimSpace(string(data)))
	}

	var decoded struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", fmt.Errorf("failed to unmarshal Ollama response: %w", err)
	}
	if decoded.Error != "" {
		return "", fmt.Errorf("ollama reported error: %s", decoded.Error)
	}
	return decoded.Response, nil
}

func resolveOllamaModel(ctx context.Context, url string) (string, error) {
	for _, path := range []string{"/api/ps", "/api/tags"} {
		model, err := firstOllamaModel(ctx, url+path)
		if err == nil && model != "" {
			return model, nil
		}
		if err != nil && path == "/api/tags" {
			// Only return the error if it's the last attempt (tags)
			return "", fmt.Errorf("ollama: no running or installed models found")
		}
	}
	return "", fmt.Errorf("ollama: no running or installed models found")
}

func firstOllamaModel(ctx context.Context, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create model lookup request: %w", err)
	}

	resp, err := ollamaHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama model lookup failed (check connectivity): %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read model lookup response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama API error (%s): %s", resp.Status, strings.TrimSpace(string(data)))
	}

	var decoded struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", fmt.Errorf("failed to unmarshal model list: %w", err)
	}
	for _, model := range decoded.Models {
		name := strings.TrimSpace(model.Model)
		if name == "" {
			name = strings.TrimSpace(model.Name)
		}
		if name != "" {
			return name, nil
		}
	}
	return "", nil
}

func (r Runner) generateCodexText(ctx context.Context, prompt string) (string, error) {
	outFile, err := os.CreateTemp("", "gitm8-commit-message-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file for Codex output: %w", err)
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)
	logging.Debug("git", "generateCodexText", "codex_output_file_created", logging.F("path", outPath))

	_, err = r.commandInputOutput(ctx, prompt, "codex", "exec",
		"--sandbox", "read-only",
		"--color", "never",
		"--ephemeral",
		"-o", outPath,
		"-",
	)
	if err != nil {
		return "", fmt.Errorf("failed to execute Codex command: %w", err)
	}

	message, err := os.ReadFile(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to read temp file after Codex execution: %w", err)
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
// forceFlag, when non-empty, is added to the push (e.g. "--force-with-lease"
// or "--force").
func (r Runner) pushOutputWithRepoURL(ctx context.Context, forceFlag string) (string, error) {
	args := []string{"push"}
	if forceFlag != "" {
		args = append(args, forceFlag)
	}
	if strings.TrimSpace(r.bestEffort(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")) == "" {
		branch := strings.TrimSpace(r.bestEffort(ctx, "branch", "--show-current"))
		remote := r.defaultRemote(ctx)
		if branch != "" && branch != "HEAD" && remote != "" {
			args = append(args, "-u", remote, branch)
		}
	}

	out, err := r.output(ctx, args...)
	url := r.repoWebURL(ctx)
	if url == "" {
		return out, err
	}
	if err != nil {
		return out, fmt.Errorf("%w\nPushed to %s", err, url)
	}
	return joinOutput(out, "Pushed to "+url) + "\n", nil
}
