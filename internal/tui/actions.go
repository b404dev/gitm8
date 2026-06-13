package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// pullRequestAction starts the GitHub CLI PR workflow and shows its output.
func (m Model) pullRequestAction() (tea.Model, tea.Cmd) {
	m.err = nil
	m.notice = ""
	provider := m.config.AIProvider
	m.gitOutput = "Asking " + provider + " to generate pull request text..."
	return m.action("Pull request ready", false, func(ctx context.Context) (string, error) {
		return m.runner.PullRequestOutput(ctx, m.config.DefaultBranch, provider)
	})
}

// manualPullRequestAction hands the terminal to gh so the user can write the PR.
func (m Model) manualPullRequestAction() (tea.Model, tea.Cmd) {
	m.err = nil
	m.notice = ""
	m.gitOutput = "Opening gh pr create..."
	return m, openManualPullRequest(m.runner.Dir, m.config.DefaultBranch)
}

// syncAction starts pull --rebase followed by push through the Git runner.
func (m Model) syncAction() (tea.Model, tea.Cmd) {
	m.err = nil
	m.notice = ""
	m.gitOutput = "Running pull --rebase, then push..."
	return m.action("Sync complete", true, func(ctx context.Context) (string, error) {
		return m.runner.SyncOutput(ctx)
	})
}

// generateCommitMessageAction asks Codex for a commit message and keeps the
// user on the commit screen when it returns.
func (m Model) generateCommitMessageAction() (tea.Model, tea.Cmd) {
	m.err = nil
	m.notice = ""
	provider := m.config.AIProvider
	m.gitOutput = "Asking " + provider + " to generate a commit message..."
	cmd := runGenerateCommitMessage(func(ctx context.Context) (string, error) {
		return m.runner.GenerateCommitMessage(ctx, provider)
	})
	if m.loading {
		return m, cmd
	}
	m.loading = true
	return m, tea.Batch(cmd, m.spinner.Tick)
}

// withNotice shows a message to the user without running a command.
func (m Model) withNotice(notice string) Model {
	m.notice = notice
	m.err = nil
	m.gitOutput = notice
	return m
}

// action starts a Git command and the spinner together. When the command
// finishes, Update receives the result and refreshes the screen if needed.
func (m Model) action(success string, refresh bool, fn func(context.Context) (string, error)) (tea.Model, tea.Cmd) {
	cmd := runGitAction(success, refresh, fn)
	if m.loading {
		return m, cmd
	}
	m.loading = true
	return m, tea.Batch(cmd, m.spinner.Tick)
}

// runGitAction runs a Git function in the way Bubble Tea expects background work.
func runGitAction(success string, refresh bool, fn func(context.Context) (string, error)) tea.Cmd {
	return func() tea.Msg {
		output, err := fn(context.Background())
		if err != nil {
			return gitActionFinishedMsg{err: err}
		}
		if strings.TrimSpace(output) == "" {
			output = success
		}
		return gitActionFinishedMsg{output: output, refresh: refresh}
	}
}

func runGenerateCommitMessage(fn func(context.Context) (string, error)) tea.Cmd {
	return func() tea.Msg {
		message, err := fn(context.Background())
		if err != nil {
			return commitMessageGeneratedMsg{err: err}
		}
		if strings.TrimSpace(message) == "" {
			return commitMessageGeneratedMsg{err: fmt.Errorf("Codex returned an empty commit message")}
		}
		return commitMessageGeneratedMsg{message: message}
	}
}

func openManualPullRequest(dir string, baseBranch string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(baseBranch) == "" {
			baseBranch = "main"
		}
		path, err := exec.LookPath("gh")
		if err != nil {
			return gitActionFinishedMsg{err: fmt.Errorf("gh CLI is required for pull requests")}
		}
		cmd := exec.Command(path, "pr", "create", "--base", baseBranch)
		if dir != "" {
			cmd.Dir = dir
		}
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return gitActionFinishedMsg{err: err}
			}
			return gitActionFinishedMsg{output: "Returned from gh pr create", refresh: true}
		})()
	}
}

// openYazi temporarily hands the terminal to yazi and refreshes after return.
func openYazi() tea.Cmd {
	return func() tea.Msg {
		path, err := exec.LookPath("yazi")
		if err != nil {
			return gitActionFinishedMsg{err: fmt.Errorf("yazi is not installed or not on PATH")}
		}
		cmd := exec.Command(path)
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return gitActionFinishedMsg{err: err}
			}
			return gitActionFinishedMsg{output: "Returned from yazi", refresh: true}
		})()
	}
}
