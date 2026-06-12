package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Main Bubble Tea Loop

// Update is where Bubble Tea sends events. It handles keys, loaded data, and
// finished Git commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ready = true
		m.width = msg.Width
		m.height = msg.Height
		m.resizeReview()
		return m, nil
	case splashTickMsg:
		return m.updateSplash(msg)
	case tea.KeyMsg:
		return m.updateKey(msg)
	case repoLoadedMsg:
		return m.handleRepoLoaded(msg), nil
	case branchesLoadedMsg:
		return m.handleBranchesLoaded(msg), nil
	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case gitActionFinishedMsg:
		return m.handleGitActionFinished(msg)
	}

	var cmd tea.Cmd
	m.review, cmd = m.review.Update(msg)
	return m, cmd
}

// Key Routing

// updateKey sends each key press to the right handler for the current screen.
func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.splash {
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	if next, cmd, handled := m.updateFocusedMode(msg); handled {
		return next, cmd
	}

	return m.updateDashboardKey(msg)
}

// updateFocusedMode sends keys to screens that temporarily take over input.
func (m Model) updateFocusedMode(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.mode {
	case "commit":
		next, cmd := m.updateCommit(msg)
		return next, cmd, true
	case "new-branch":
		next, cmd := m.updateNewBranch(msg)
		return next, cmd, true
	case "delete-branch":
		next, cmd := m.updateDeleteBranch(msg)
		return next, cmd, true
	case "branches":
		next, cmd := m.updateBranches(msg)
		return next, cmd, true
	case "profiles":
		next, cmd := m.updateProfiles(msg)
		return next, cmd, true
	case "help":
		next, cmd := m.updateHelp(msg)
		return next, cmd, true
	case "rebase":
		next, cmd := m.updateRebase(msg)
		return next, cmd, true
	}

	return m, nil, false
}

// updateDashboardKey handles keys used by the normal dashboard screen.
func (m Model) updateDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "o":
		m.outputExpanded = !m.outputExpanded
		return m, nil
	case "tab":
		m.footerHidden = !m.footerHidden
		return m, nil
	case "x":
		return m.syncAction()
	case "r", "ctrl+p":
		return m.pullRequestAction()
	case "q", "ctrl+c":
		return m, tea.Quit
	case "R", "ctrl+r":
		m.mode = "rebase"
		m.notice = ""
		m.err = nil
		return m, loadRebase(m.runner)
	case "l":
		m.target = "repo"
		m.mode = "logs"
		m.notice = ""
		m.err = nil
		return m, loadLog(m.runner, m.config.ShowCommitGraph)
	case "0":
		m.target = "repo"
		m.mode = "review"
		return m, loadReview(m.runner, "")
	case "enter":
		if len(m.files) == 0 {
			return m.withNotice("No changed files"), nil
		}
		m.target = m.files[m.fileCursor].Path
		m.mode = "preview"
		return m, loadPreview(m.runner, m.target)
	case "f":
		return m.action("Fetch complete", true, func(ctx context.Context) (string, error) {
			return m.runner.FetchOutput(ctx)
		})
	case "p":
		return m.action("Pull complete", true, func(ctx context.Context) (string, error) {
			return m.runner.PullOutput(ctx)
		})
	case "P":
		return m.action("Push complete", true, func(ctx context.Context) (string, error) {
			return m.runner.PushOutput(ctx)
		})
	case "d":
		return m.diffOrPreviewSelected()
	case "b":
		m.mode = "branches"
		return m, loadBranches(m.runner)
	case "i":
		m.mode = "profiles"
		m.reconcileProfileCursor()
		m.notice = ""
		m.err = nil
		m.review.SetContent(m.profilesView())
		m.review.GotoTop()
		return m, nil
	case "h":
		m.mode = "help"
		m.notice = ""
		m.err = nil
		m.review.SetContent(m.helpView())
		m.review.GotoTop()
		return m, nil
	case "s":
		path := m.selectedPath()
		if path == "" {
			return m.withNotice("Select a file before staging"), nil
		}
		return m.action("Staged "+path, true, func(ctx context.Context) (string, error) {
			return m.runner.StageOutput(ctx, path)
		})
	case "S":
		return m.action("Staged all changes", true, func(ctx context.Context) (string, error) {
			return m.runner.StageAllOutput(ctx)
		})
	case "u":
		path := m.selectedPath()
		if path == "" {
			return m.withNotice("Select a file before unstaging"), nil
		}
		return m.action("Unstaged "+path, true, func(ctx context.Context) (string, error) {
			return m.runner.UnstageOutput(ctx, path)
		})
	case "U":
		return m.action("Unstaged all changes", true, func(ctx context.Context) (string, error) {
			return m.runner.UnstageAllOutput(ctx)
		})
	case "c":
		m.mode = "commit"
		m.commit.SetValue("")
		m.commit.Focus()
		m.notice = ""
		m.err = nil
		return m, nil
	case "y":
		return m, openYazi()
	case "down":
		m = m.moveFileCursor(1)
		if len(m.files) == 0 {
			return m, nil
		}
		return m, loadPreview(m.runner, m.target)
	case "up":
		m = m.moveFileCursor(-1)
		if len(m.files) == 0 {
			return m, nil
		}
		return m, loadPreview(m.runner, m.target)
	}

	return m.updateViewportKey(msg)
}

// Shared Viewer Keys

// updateViewportKey handles scrolling keys used by review, preview, log, and help.
func (m Model) updateViewportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j":
		m.review.LineDown(1)
	case "k":
		m.review.LineUp(1)
	case "pgdown", "ctrl+f":
		m.review.ViewDown()
	case "pgup", "ctrl+b":
		m.review.ViewUp()
	case "g":
		m.review.GotoTop()
	case "G":
		m.review.GotoBottom()
	}
	return m, nil
}

// Dashboard View Switching

// diffOrPreviewSelected toggles the selected file between diff and content views.
func (m Model) diffOrPreviewSelected() (tea.Model, tea.Cmd) {
	if len(m.files) == 0 {
		m.target = "repo"
		m.mode = "review"
		return m, loadReview(m.runner, "")
	}

	file := m.files[m.fileCursor]
	if m.mode == "review" && m.target == file.Path {
		m.mode = "preview"
		return m, loadPreview(m.runner, file.Path)
	}

	m.target = file.Path
	if file.Index == '?' {
		m.mode = "preview"
		return m, loadPreview(m.runner, file.Path)
	}
	m.mode = "review"
	return m, loadReview(m.runner, file.Path)
}

// Messages From Background Work

// handleRepoLoaded stores newly loaded repo data and updates the main viewer.
func (m Model) handleRepoLoaded(msg repoLoadedMsg) Model {
	m.info = msg.info
	m.files = msg.files
	m.target = msg.target
	m.mode = msg.mode
	m.reconcileFileCursor()
	m.err = msg.err
	m.notice = ""

	review := msg.review
	if msg.mode == "preview" {
		review = highlightPreview(msg.target, review)
	}
	m.review.SetContent(review)
	m.review.GotoTop()
	return m
}

// handleBranchesLoaded stores branch data and redraws the branch or rebase picker.
func (m Model) handleBranchesLoaded(msg branchesLoadedMsg) Model {
	m.info = msg.info
	m.files = msg.files
	m.branches = msg.branches
	m.mode = msg.mode
	m.reconcileFileCursor()
	m.reconcileBranchCursor()
	m.err = msg.err
	m.notice = ""

	if m.mode == "rebase" {
		m.review.SetContent(m.rebaseView())
	} else {
		m.review.SetContent(m.branchesView())
	}
	m.review.GotoTop()
	return m
}

// handleGitActionFinished stores the result of a Git command and reloads the screen when needed.
func (m Model) handleGitActionFinished(msg gitActionFinishedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.err = msg.err
	if msg.err != nil {
		m.notice = ""
		m.gitOutput = msg.err.Error()
		return m, nil
	}

	m.notice = msg.output
	m.gitOutput = msg.output
	if msg.refresh {
		return m, loadCurrent(m.runner, m.selectedPath(), m.mode, m.config.ShowCommitGraph)
	}
	if m.mode == "branches" {
		return m, loadBranches(m.runner)
	}
	if strings.TrimSpace(msg.output) != "" {
		m.review.SetContent(msg.output)
		m.review.GotoTop()
	}
	return m, nil
}

// Commit Screen Keys

// updateCommit handles typing a commit message and pressing enter to commit.
func (m Model) updateCommit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "review"
		m.commit.Blur()
		m.notice = "Commit cancelled"
		return m, nil
	case "enter":
		message := strings.TrimSpace(m.commit.Value())
		if message == "" {
			m.err = fmt.Errorf("commit message is required")
			return m, nil
		}
		m.commit.Blur()
		m.mode = "review"
		return m.action("Committed changes", true, func(ctx context.Context) (string, error) {
			return m.runner.CommitOutput(ctx, message)
		})
	}

	var cmd tea.Cmd
	m.commit, cmd = m.commit.Update(msg)
	return m, cmd
}

// Branch Screen Keys

// updateBranches handles keys while the branch picker is open.
func (m Model) updateBranches(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "review"
		return m, loadReview(m.runner, m.selectedPath())
	case "r":
		return m, loadBranches(m.runner)
	case "q", "ctrl+c":
		return m, tea.Quit
	case "n":
		m.mode = "new-branch"
		m.branchInput.SetValue("")
		m.branchInput.Focus()
		m.notice = ""
		m.err = nil
		return m, nil
	case "D":
		if len(m.branches) == 0 {
			return m.withNotice("No branches found"), nil
		}
		if m.branches[m.branchCursor] == m.info.Branch {
			return m.withNotice("Cannot delete the current branch"), nil
		}
		m.mode = "delete-branch"
		m.notice = "Delete branch " + m.branches[m.branchCursor] + "?"
		m.err = nil
		return m, nil
	case "down":
		m = m.moveBranchCursor(1)
		m.review.SetContent(m.branchesView())
		return m, nil
	case "up":
		m = m.moveBranchCursor(-1)
		m.review.SetContent(m.branchesView())
		return m, nil
	case "enter":
		if len(m.branches) == 0 {
			return m.withNotice("No branches found"), nil
		}
		branch := m.branches[m.branchCursor]
		m.mode = "review"
		return m.action("Switched to "+branch, true, func(ctx context.Context) (string, error) {
			return m.runner.SwitchBranchOutput(ctx, branch)
		})
	case "W":
		if len(m.branches) == 0 {
			return m.withNotice("No branches found"), nil
		}
		branch := m.branches[m.branchCursor]
		m.mode = "review"
		return m.action("Switched to "+branch+" with changes", true, func(ctx context.Context) (string, error) {
			return m.runner.SwitchBranchWithChangesOutput(ctx, branch)
		})
	}
	return m, nil
}

// Rebase Screen Keys

// updateRebase handles choosing a rebase target and continue/abort/skip keys.
func (m Model) updateRebase(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "review"
		return m, loadReview(m.runner, m.selectedPath())
	case "q", "ctrl+c":
		return m, tea.Quit
	case "down":
		m = m.moveBranchCursor(1)
		m.review.SetContent(m.rebaseView())
		return m, nil
	case "up":
		m = m.moveBranchCursor(-1)
		m.review.SetContent(m.rebaseView())
		return m, nil
	case "c":
		m.mode = "review"
		return m.action("Rebase continued", true, func(ctx context.Context) (string, error) {
			return m.runner.RebaseContinueOutput(ctx)
		})
	case "a":
		m.mode = "review"
		return m.action("Rebase aborted", true, func(ctx context.Context) (string, error) {
			return m.runner.RebaseAbortOutput(ctx)
		})
	case "s":
		m.mode = "review"
		return m.action("Rebase skipped current patch", true, func(ctx context.Context) (string, error) {
			return m.runner.RebaseSkipOutput(ctx)
		})
	case "enter":
		if len(m.branches) == 0 {
			return m.withNotice("No branches found"), nil
		}
		target := m.branches[m.branchCursor]
		if target == m.info.Branch {
			return m.withNotice("Cannot rebase onto the current branch"), nil
		}
		m.mode = "review"
		return m.action("Rebased "+m.info.Branch+" onto "+target, true, func(ctx context.Context) (string, error) {
			return m.runner.RebaseOutput(ctx, target)
		})
	}
	return m, nil
}

// Delete Branch Prompt Keys

// updateDeleteBranch handles the delete prompt: local only or local plus remote.
func (m Model) updateDeleteBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.mode = "branches"
		m.notice = "Delete cancelled"
		return m, nil
	case "l":
		return m.deleteBranch(false)
	case "r":
		return m.deleteBranch(true)
	}
	return m, nil
}

// deleteBranch keeps the prompt simple: local-only or local plus remote.
func (m Model) deleteBranch(includeRemote bool) (tea.Model, tea.Cmd) {
	if len(m.branches) == 0 {
		m.mode = "branches"
		return m.withNotice("No branches found"), nil
	}
	branch := m.branches[m.branchCursor]
	m.mode = "branches"
	if includeRemote {
		return m.action("Deleted "+branch+" (local + remote)", true, func(ctx context.Context) (string, error) {
			local, localErr := m.runner.DeleteBranchOutput(ctx, branch)
			remote, remoteErr := m.runner.DeleteRemoteBranchOutput(ctx, branch)
			out := joinGitOutput(local, remote)
			if remoteErr != nil {
				out = joinGitOutput(out, "remote: "+remoteErr.Error())
			}
			return out, localErr
		})
	}
	return m.action("Deleted "+branch+" (local)", true, func(ctx context.Context) (string, error) {
		return m.runner.DeleteBranchOutput(ctx, branch)
	})
}

// New Branch Screen Keys

// updateNewBranch handles typing a new branch name and creating it.
func (m Model) updateNewBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "branches"
		m.branchInput.Blur()
		m.notice = "Branch creation cancelled"
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.branchInput.Value())
		if name == "" {
			m.err = fmt.Errorf("branch name is required")
			return m, nil
		}
		m.branchInput.Blur()
		m.mode = "review"
		return m.action("Created and switched to "+name, true, func(ctx context.Context) (string, error) {
			created, err := m.runner.CreateBranchOutput(ctx, name)
			if err != nil {
				return created, err
			}
			upstream, err := m.runner.SetUpstreamOutput(ctx, "origin", name)
			return joinGitOutput(created, upstream), err
		})
	}

	var cmd tea.Cmd
	m.branchInput, cmd = m.branchInput.Update(msg)
	return m, cmd
}

// Identity Profile Screen Keys

// updateProfiles handles moving through identity profiles and choosing one.
func (m Model) updateProfiles(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "review"
		return m, loadReview(m.runner, m.selectedPath())
	case "q", "ctrl+c":
		return m, tea.Quit
	case "down":
		m = m.moveProfileCursor(1)
		m.review.SetContent(m.profilesView())
		return m, nil
	case "up":
		m = m.moveProfileCursor(-1)
		m.review.SetContent(m.profilesView())
		return m, nil
	case "enter":
		return m.applySelectedProfile()
	}
	return m, nil
}

// applySelectedProfile writes the selected identity into this repo's Git config.
func (m Model) applySelectedProfile() (tea.Model, tea.Cmd) {
	if len(m.config.Profiles) == 0 {
		return m.withNotice("No profiles configured (see ~/.gitm8/profiles)"), nil
	}
	profile := m.config.Profiles[m.profileCursor]
	m.mode = "review"
	return m.action("Switched identity to "+profile.Label, true, func(ctx context.Context) (string, error) {
		return m.runner.SetUserOutput(ctx, profile.Name, profile.Email)
	})
}

// Help Screen Keys

// updateHelp lets help scroll while escape or h returns to the main screen.
func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "h":
		m.mode = "review"
		return m, loadReview(m.runner, m.selectedPath())
	default:
		return m.updateViewportKey(msg)
	}
}
