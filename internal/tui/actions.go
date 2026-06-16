package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/git"
	"github.com/b404dev/gitm8/internal/logging"
)

// pullRequestAction starts the GitHub CLI PR workflow and shows its output.
func (m Model) pullRequestAction() (tea.Model, tea.Cmd) {
	m.err = nil
	m.notice = ""
	provider := m.config.AIProvider
	if !m.config.AIAvailable {
		reason := m.aiUnavailableNotice()
		logging.Warn("tui", "pullRequestAction", "generated_pr_unavailable", logging.F("provider", provider), logging.F("reason", reason))
		return m.withNotice(reason), nil
	}
	m.gitOutput = "Asking " + provider + " to generate pull request text..."
	logging.Info("tui", "pullRequestAction", "generated_pr_selected", logging.F("provider", provider), logging.F("base", m.config.DefaultBranch))
	return m.action("Pull request ready", false, func(ctx context.Context) (string, error) {
		return m.runner.PullRequestOutput(ctx, m.config.DefaultBranch, provider, m.config.OllamaURL)
	})
}

// manualPullRequestAction hands the terminal to gh so the user can write the PR.
func (m Model) manualPullRequestAction() (tea.Model, tea.Cmd) {
	m.err = nil
	m.notice = ""
	m.gitOutput = "Opening gh pr create..."
	logging.Info("tui", "manualPullRequestAction", "manual_pr_selected", logging.F("base", m.config.DefaultBranch))
	return m, openManualPullRequest(m.runner.ManualPullRequestCommand, m.config.DefaultBranch)
}

// pushAction pushes the current branch. If the remote refuses it as a
// non-fast-forward (the usual outcome after a squash rewrote history), the
// result asks the user whether to retry with a force-with-lease push.
func (m Model) pushAction() (tea.Model, tea.Cmd) {
	m.err = nil
	m.notice = ""
	if m.config.MattMode {
		m.gitOutput = "Force pushing (matt_mode)..."
		logging.Info("tui", "pushAction", "push_start", logging.F("matt_mode", true))
		return m.action("Force pushed with --force (matt_mode)", true, func(ctx context.Context) (string, error) {
			return m.runner.ForcePushUnconditionalOutput(ctx)
		})
	}
	m.gitOutput = "Pushing..."
	logging.Info("tui", "pushAction", "push_start")
	cmd := runPush(m.runner.PushOutput)
	if m.loading {
		return m, cmd
	}
	m.loading = true
	return m, tea.Batch(cmd, m.spinner.Tick)
}

// runPush runs a push and reports whether a non-fast-forward rejection occurred.
func runPush(fn func(context.Context) (string, error)) tea.Cmd {
	return func() tea.Msg {
		out, err := fn(context.Background())
		if err != nil {
			return pushFinishedMsg{err: err, rejected: git.PushRejectedNonFastForward(err)}
		}
		if strings.TrimSpace(out) == "" {
			out = "Push complete"
		}
		return pushFinishedMsg{output: out}
	}
}

// generateCommitMessageAction asks Codex for a commit message and keeps the
// user on the commit screen when it returns.
func (m Model) generateCommitMessageAction() (tea.Model, tea.Cmd) {
	m.err = nil
	m.notice = ""
	provider := m.config.AIProvider
	if !m.config.AIAvailable {
		reason := m.aiUnavailableNotice()
		logging.Warn("tui", "generateCommitMessageAction", "commit_message_generate_unavailable", logging.F("provider", provider), logging.F("reason", reason))
		return m.withNotice(reason), nil
	}
	m.gitOutput = "Asking " + provider + " to generate a commit message..."
	logging.Info("tui", "generateCommitMessageAction", "commit_message_generate_start", logging.F("provider", provider))
	cmd := runGenerateCommitMessage(func(ctx context.Context) (string, error) {
		return m.runner.GenerateCommitMessage(ctx, provider, m.config.OllamaURL)
	})
	if m.loading {
		return m, cmd
	}
	m.loading = true
	return m, tea.Batch(cmd, m.spinner.Tick)
}

func (m Model) aiUnavailableNotice() string {
	reason := strings.TrimSpace(m.config.AIUnavailableReason)
	if reason == "" {
		reason = m.config.AIProvider + " is not installed or not on PATH"
	}
	return "AI generation unavailable: " + reason
}

// withNotice shows a message to the user without running a command.
func (m Model) withNotice(notice string) Model {
	m.notice = notice
	m.err = nil
	m.gitOutput = notice
	logging.Info("tui", "withNotice", "notice", logging.F("message", notice))
	return m
}

// action starts a Git command and the spinner together. When the command
// finishes, Update receives the result and refreshes the screen if needed.
func (m Model) action(success string, refresh bool, fn func(context.Context) (string, error)) (tea.Model, tea.Cmd) {
	cmd := runGitAction(success, refresh, fn)
	if m.loading {
		logging.Warn("tui", "action", "action_started_while_loading", logging.F("success", success), logging.F("refresh", refresh))
		return m, cmd
	}
	m.loading = true
	logging.Info("tui", "action", "action_start", logging.F("success", success), logging.F("refresh", refresh))
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
			return commitMessageGeneratedMsg{err: fmt.Errorf("AI returned an empty commit message")}
		}
		return commitMessageGeneratedMsg{message: message}
	}
}

func (m Model) openSelectedInEditor() (tea.Model, tea.Cmd) {
	path := m.selectedPath()
	if path == "" {
		return m.withNotice("Select a file before opening the editor"), nil
	}
	editor := strings.TrimSpace(m.config.Editor)
	if editor == "" {
		editor = "vi"
	}
	m.err = nil
	m.notice = ""
	m.gitOutput = "Opening " + path + " in " + editor + "..."
	logging.Info("tui", "openSelectedInEditor", "editor_open_requested", logging.F("editor", editor), logging.F("path", path))
	return m, openEditor(editor, path)
}

func openEditor(editor string, filePath string) tea.Cmd {
	return func() tea.Msg {
		cmd, err := editorCommand(editor, filePath)
		if err != nil {
			logging.Error("tui", "openEditor", "editor_command_failed", logging.F("editor", editor), logging.F("path", filePath), logging.F("error", err))
			return gitActionFinishedMsg{err: err}
		}
		logging.Info("tui", "openEditor", "editor_exec_start", logging.F("editor", editor), logging.F("path", filePath))
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				logging.Error("tui", "openEditor", "editor_exec_failed", logging.F("editor", editor), logging.F("path", filePath), logging.F("error", err))
				return gitActionFinishedMsg{err: err}
			}
			logging.Info("tui", "openEditor", "editor_exec_complete", logging.F("editor", editor), logging.F("path", filePath))
			return gitActionFinishedMsg{output: "Returned from " + editor, refresh: true}
		})()
	}
}

func editorCommand(editor string, filePath string) (*exec.Cmd, error) {
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return nil, fmt.Errorf("editor is not configured")
	}
	if len(fields) == 1 {
		path, err := exec.LookPath(fields[0])
		if err != nil {
			return nil, fmt.Errorf("editor %q is not installed or not on PATH", fields[0])
		}
		return exec.Command(path, filePath), nil
	}
	if runtime.GOOS == "windows" {
		args := []string{"/C", editor, filePath}
		return exec.Command("cmd", args...), nil
	}
	return exec.Command("sh", "-c", editor+" \"$1\"", "gitm8-editor", filePath), nil
}

func openManualPullRequest(fn func(string) (*exec.Cmd, error), baseBranch string) tea.Cmd {
	return func() tea.Msg {
		cmd, err := fn(baseBranch)
		if err != nil {
			logging.Error("tui", "openManualPullRequest", "manual_pr_command_failed", logging.F("error", err))
			return gitActionFinishedMsg{err: err}
		}
		logging.Info("tui", "openManualPullRequest", "manual_pr_exec_start", logging.F("base", baseBranch))
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				logging.Error("tui", "openManualPullRequest", "manual_pr_exec_failed", logging.F("error", err))
				return gitActionFinishedMsg{err: err}
			}
			logging.Info("tui", "openManualPullRequest", "manual_pr_exec_complete")
			return gitActionFinishedMsg{output: "Returned from gh pr create", refresh: true}
		})()
	}
}

// openYazi temporarily hands the terminal to yazi and refreshes after return.
func openYazi() tea.Cmd {
	return func() tea.Msg {
		path, err := exec.LookPath("yazi")
		if err != nil {
			logging.Error("tui", "openYazi", "yazi_missing", logging.F("error", err))
			return gitActionFinishedMsg{err: fmt.Errorf("yazi is not installed or not on PATH")}
		}
		cmd := exec.Command(path)
		logging.Info("tui", "openYazi", "yazi_exec_start")
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				logging.Error("tui", "openYazi", "yazi_exec_failed", logging.F("error", err))
				return gitActionFinishedMsg{err: err}
			}
			logging.Info("tui", "openYazi", "yazi_exec_complete")
			return gitActionFinishedMsg{output: "Returned from yazi", refresh: true}
		})()
	}
}
