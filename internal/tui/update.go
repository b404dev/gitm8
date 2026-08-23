package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/git"
	"github.com/b404dev/gitm8/internal/logging"
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
		logging.Debug("tui", "Update", "window_resized", logging.F("width", msg.Width), logging.F("height", msg.Height))
		return m, nil
	case splashTickMsg:
		return m.updateSplash(msg)
	case tea.KeyMsg:
		return m.updateKey(msg)
	case repoLoadedMsg:
		// A repository load started before the user opened the project picker
		// must not pull the UI back into a "no repo" dashboard when it finishes.
		if m.mode == "workspace" {
			return m, nil
		}
		return m.handleRepoLoaded(msg), nil
	case branchesLoadedMsg:
		return m.handleBranchesLoaded(msg), nil
	case conflictsLoadedMsg:
		return m.handleConflictsLoaded(msg), nil
	case stashesLoadedMsg:
		return m.handleStashesLoaded(msg), nil
	case squashLoadedMsg:
		return m.handleSquashLoaded(msg), nil
	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case gitActionFinishedMsg:
		return m.handleGitActionFinished(msg)
	case pushFinishedMsg:
		return m.handlePushFinished(msg)
	case commitMessageGeneratedMsg:
		return m.handleCommitMessageGenerated(msg), nil
	case setupFinishedMsg:
		return m.handleSetupFinished(msg)
	case setupAuthFinishedMsg:
		return m.handleSetupAuthFinished(msg)
	case projectsLoadedMsg:
		m.projects, m.err = msg.projects, msg.err
		m.projectCursor = clamp(m.projectCursor, 0, max(0, len(m.projects)-1))
		return m, nil
	case projectOpenedMsg:
		return m.handleProjectOpened(msg)
	}

	var cmd tea.Cmd
	m.review, cmd = m.review.Update(msg)
	return m, cmd
}

// Key Routing

// updateKey sends each key press to the right handler for the current screen.
func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	logging.Debug("tui", "updateKey", "key_received", logging.F("key", msg.String()), logging.F("mode", m.mode), logging.F("target", m.target))
	if m.splash {
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			logging.Info("tui", "updateKey", "quit_from_splash", logging.F("key", msg.String()))
			return m, tea.Quit
		}
		return m, nil
	}
	if m.mode == "setup" {
		return m.updateSetup(msg)
	}
	if m.mode == "workspace" {
		return m.updateWorkspace(msg)
	}

	if m.fileFilterActive {
		return m.updateFileFilter(msg)
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
	case "discard-file":
		next, cmd := m.updateDiscardFile(msg)
		return next, cmd, true
	case "branches":
		next, cmd := m.updateBranches(msg)
		return next, cmd, true
	case "profiles":
		next, cmd := m.updateProfiles(msg)
		return next, cmd, true
	case "pull-request":
		next, cmd := m.updatePullRequest(msg)
		return next, cmd, true
	case "help":
		next, cmd := m.updateHelp(msg)
		return next, cmd, true
	case "rebase":
		next, cmd := m.updateRebase(msg)
		return next, cmd, true
	case "conflicts":
		next, cmd := m.updateConflicts(msg)
		return next, cmd, true
	case "stashes":
		next, cmd := m.updateStashes(msg)
		return next, cmd, true
	case "squash":
		next, cmd := m.updateSquash(msg)
		return next, cmd, true
	case "force-push":
		next, cmd := m.updateForcePush(msg)
		return next, cmd, true
	case "search":
		next, cmd := m.updateSearch(msg)
		return next, cmd, true
	}

	return m, nil, false
}

// updateDashboardKey handles keys used by the normal dashboard screen.
func (m Model) updateDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	logging.Info("tui", "updateDashboardKey", "key_pressed", logging.F("key", msg.String()), logging.F("mode", m.mode), logging.F("target", m.target))
	switch msg.String() {
	case "w":
		return m.openWorkspace()
	case "o":
		m.outputExpanded = !m.outputExpanded
		logging.Info("tui", "updateDashboardKey", "output_toggle", logging.F("expanded", m.outputExpanded))
		return m, nil
	case "tab":
		m.footerHidden = !m.footerHidden
		logging.Info("tui", "updateDashboardKey", "footer_toggle", logging.F("hidden", m.footerHidden))
		return m, nil
	case "/":
		if m.mode == "preview" {
			return m.startSearch()
		}
		return m.startFileFilter()
	case "z":
		if m.info.Branch == "" || m.info.Branch == "detached" {
			return m.withNotice("Cannot squash commits while detached"), nil
		}
		if m.info.Branch == m.config.DefaultBranch {
			return m.withNotice("Cannot squash commits on the default branch"), nil
		}
		m.mode = "squash"
		m.notice = ""
		m.err = nil
		return m, loadSquash(m.runner, m.config.DefaultBranch)
	case "r":
		m.mode = "pull-request"
		m.notice = ""
		m.err = nil
		m.review.SetContent(m.pullRequestView())
		m.review.GotoTop()
		logging.Info("tui", "updateDashboardKey", "pull_request_options_opened")
		return m, nil
	case "q", "ctrl+c":
		logging.Info("tui", "updateDashboardKey", "quit", logging.F("key", msg.String()))
		return m, tea.Quit
	case "R":
		m.mode = "rebase"
		m.notice = ""
		m.err = nil
		return m, loadRebase(m.runner)
	case "C":
		m.mode = "conflicts"
		m.notice = ""
		m.err = nil
		return m, loadConflicts(m.runner, m.conflictCursor)
	case "t":
		m.mode = "stashes"
		m.notice = ""
		m.err = nil
		return m, loadStashes(m.runner, m.stashCursor)
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
		files := m.filteredFiles()
		if len(files) == 0 {
			return m.withNotice("No changed files"), nil
		}
		if m.target == files[m.fileCursor].Path && (m.mode == "preview" || m.mode == "review") {
			return m.openSelectedInEditor()
		}
		m.target = files[m.fileCursor].Path
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
		return m.pushAction()
	case "d":
		return m.diffOrPreviewSelected()
	case "x":
		file, ok := m.selectedFile()
		if !ok {
			return m.withNotice("Select a file before discarding changes"), nil
		}
		if !m.config.ConfirmDestructiveActions {
			return m.discardFile(file)
		}
		m.mode = "discard-file"
		m.notice = "Discard all changes to " + file.DisplayPath() + "?"
		m.err = nil
		return m, nil
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
		file, ok := m.selectedFile()
		if !ok {
			return m.withNotice("Select a file before staging"), nil
		}
		return m.action(stageNotice(file), true, func(ctx context.Context) (string, error) {
			return m.runner.StageOutput(ctx, file.GitPaths()...)
		})
	case "S":
		return m.action("Staged all changes", true, func(ctx context.Context) (string, error) {
			return m.runner.StageAllOutput(ctx)
		})
	case "u":
		file, ok := m.selectedFile()
		if !ok {
			return m.withNotice("Select a file before unstaging"), nil
		}
		return m.action(unstageNotice(file), true, func(ctx context.Context) (string, error) {
			return m.runner.UnstageOutput(ctx, file.GitPaths()...)
		})
	case "U":
		return m.action("Unstaged all changes", true, func(ctx context.Context) (string, error) {
			return m.runner.UnstageAllOutput(ctx)
		})
	case "n":
		file, ok := m.selectedFile()
		if !ok {
			return m.withNotice("Select a file before stashing"), nil
		}
		return m.action("Stashed "+file.DisplayPath(), true, func(ctx context.Context) (string, error) {
			return m.runner.StashPushPathsOutput(ctx, file.GitPaths()...)
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
		if len(m.filteredFiles()) == 0 {
			return m, nil
		}
		return m, loadPreview(m.runner, m.target)
	case "up":
		m = m.moveFileCursor(-1)
		if len(m.filteredFiles()) == 0 {
			return m, nil
		}
		return m, loadPreview(m.runner, m.target)
	}

	return m.updateViewportKey(msg)
}

// selectedFile returns the file row currently highlighted in the files panel.
func (m Model) selectedFile() (git.FileStatus, bool) {
	files := m.filteredFiles()
	if len(files) == 0 {
		return git.FileStatus{}, false
	}
	cursor := clamp(m.fileCursor, 0, len(files)-1)
	return files[cursor], true
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
	logging.Info("tui", "handleRepoLoaded", "repo_loaded", logging.F("mode", msg.mode), logging.F("target", msg.target), logging.F("files", len(msg.files)), logging.F("error", msg.err))
	m.info = msg.info
	m.files = msg.files
	m.target = msg.target
	m.mode = msg.mode
	m.viewerContent = msg.review
	m.reconcileFileCursor()
	m.err = msg.err
	m.notice = msg.notice

	review := msg.review
	if msg.mode == "preview" {
		review = highlightPreview(msg.target, review)
	}
	m.review.SetContent(review)
	m.review.GotoTop()
	return m
}

// startSearch opens a focused fuzzy finder for the current file preview.
func (m Model) startSearch() (tea.Model, tea.Cmd) {
	if m.mode != "preview" || m.selectedPath() == "" {
		return m.withNotice("Open a file preview before searching"), nil
	}
	if strings.TrimSpace(m.viewerContent) == "" {
		return m.withNotice("Nothing to search in this preview"), nil
	}
	m.searchReturn = m.mode
	m.searchYOffset = m.review.YOffset
	m.mode = "search"
	m.searchInput.SetValue("")
	m.searchInput.Focus()
	m.searchMatches = nil
	m.searchCursor = 0
	m.searchOffset = 0
	m.notice = ""
	m.err = nil
	m.review.SetContent(m.searchView())
	m.review.SetYOffset(m.searchYOffset)
	return m, nil
}

// startFileFilter opens a focused filter for the changed-files panel.
func (m Model) startFileFilter() (tea.Model, tea.Cmd) {
	if len(m.files) == 0 {
		return m.withNotice("No changed files to filter"), nil
	}
	m.fileFilterActive = true
	m.fileFilter.Focus()
	m.fileCursor = 0
	m.fileOffset = 0
	m.reconcileFileCursor()
	m.notice = ""
	m.err = nil
	return m, nil
}

// updateFileFilter lets the user narrow the changed-files list while typing.
func (m Model) updateFileFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.fileFilterActive = false
		m.fileFilter.Blur()
		m.notice = "File filter applied"
		m.err = nil
		return m, nil
	case "esc":
		m.fileFilterActive = false
		m.fileFilter.Blur()
		m.fileFilter.SetValue("")
		m.reconcileFileCursor()
		m.notice = "File filter cleared"
		m.err = nil
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}

	old := m.fileFilter.Value()
	var cmd tea.Cmd
	m.fileFilter, cmd = m.fileFilter.Update(msg)
	if m.fileFilter.Value() != old {
		m.fileCursor = 0
		m.fileOffset = 0
		m.reconcileFileCursor()
	}
	return m, cmd
}

// updateSearch lets the user fuzzy-find lines inside the current file preview.
func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		return m.closeSearch(false), nil
	case "q", "ctrl+c":
		return m, tea.Quit
	case "enter":
		if len(m.searchMatches) == 0 {
			return m, nil
		}
		return m.closeSearch(true), nil
	case "down":
		if len(m.searchMatches) > 0 {
			m.searchCursor = clamp(m.searchCursor+1, 0, len(m.searchMatches)-1)
			m.review.SetContent(m.searchView())
			m.scrollToSearchMatch()
		}
		return m, nil
	case "up":
		if len(m.searchMatches) > 0 {
			m.searchCursor = clamp(m.searchCursor-1, 0, len(m.searchMatches)-1)
			m.review.SetContent(m.searchView())
			m.scrollToSearchMatch()
		}
		return m, nil
	}

	old := m.searchInput.Value()
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	if m.searchInput.Value() != old {
		m.searchCursor = 0
		m.searchOffset = 0
	}
	m.refreshSearchResults()
	m.review.SetContent(m.searchView())
	if len(m.searchMatches) > 0 {
		m.scrollToSearchMatch()
	} else {
		m.review.SetYOffset(m.searchYOffset)
	}
	return m, cmd
}

func (m Model) closeSearch(jump bool) Model {
	offset := m.searchYOffset
	if jump && len(m.searchMatches) > 0 {
		offset = max(0, m.searchMatches[m.searchCursor].LineIndex-2)
		m.notice = fmt.Sprintf("Jumped to line %d", m.searchMatches[m.searchCursor].LineNo)
	} else {
		m.notice = "Search cancelled"
	}
	m.searchInput.Blur()
	if m.searchReturn == "" {
		m.searchReturn = "preview"
	}
	m.mode = m.searchReturn
	m.review.SetContent(highlightPreview(m.target, m.viewerContent))
	m.review.SetYOffset(offset)
	m.err = nil
	return m
}

func (m *Model) scrollToSearchMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.review.SetYOffset(max(0, m.searchMatches[m.searchCursor].LineIndex-2))
}

// handleBranchesLoaded stores branch data and redraws the branch or rebase picker.
func (m Model) handleBranchesLoaded(msg branchesLoadedMsg) Model {
	logging.Info("tui", "handleBranchesLoaded", "branches_loaded", logging.F("mode", msg.mode), logging.F("branches", len(msg.branches)), logging.F("files", len(msg.files)), logging.F("error", msg.err))
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
		logging.Error("tui", "handleGitActionFinished", "action_failed", logging.F("error", msg.err))
		m.notice = ""
		m.gitOutput = msg.err.Error()
		return m, nil
	}

	logging.Info("tui", "handleGitActionFinished", "action_complete", logging.F("refresh", msg.refresh), logging.F("output_bytes", len(msg.output)))
	m.notice = msg.output
	m.gitOutput = msg.output
	if m.mode == "branches" {
		return m, loadBranches(m.runner)
	}
	if m.mode == "conflicts" {
		return m, loadConflicts(m.runner, m.conflictCursor)
	}
	if m.mode == "stashes" {
		return m, loadStashesWithNotice(m.runner, m.stashCursor, msg.output)
	}
	if msg.refresh {
		return m, loadCurrentWithNotice(m.runner, m.selectedPath(), m.mode, m.config.ShowCommitGraph, msg.output)
	}
	if strings.TrimSpace(msg.output) != "" {
		m.review.SetContent(msg.output)
		m.review.GotoTop()
	}
	return m, nil
}

// handlePushFinished records a push result and, when the remote rejected the
// push as a non-fast-forward, opens the force-push confirmation prompt.
func (m Model) handlePushFinished(msg pushFinishedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.err = msg.err
		m.gitOutput = msg.err.Error()
		m.notice = ""
		if msg.rejected {
			m.mode = "force-push"
			m.notice = "Push rejected — remote history differs (expected after a squash). Force push?"
			logging.Info("tui", "handlePushFinished", "push_rejected_offer_force")
			return m, nil
		}
		logging.Error("tui", "handlePushFinished", "push_failed", logging.F("error", msg.err))
		return m, nil
	}

	m.err = nil
	m.notice = msg.output
	m.gitOutput = msg.output
	logging.Info("tui", "handlePushFinished", "push_complete")
	return m, loadCurrent(m.runner, m.selectedPath(), m.mode, m.config.ShowCommitGraph)
}

// updateForcePush confirms before retrying a rejected push with force-with-lease.
func (m Model) updateForcePush(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.mode = "review"
		m.notice = "Force push cancelled"
		logging.Info("tui", "updateForcePush", "force_push_cancelled")
		return m, loadReview(m.runner, m.selectedPath())
	case "q", "ctrl+c":
		return m, tea.Quit
	case "y":
		m.mode = "review"
		logging.Info("tui", "updateForcePush", "force_push_requested")
		return m.action("Force pushed with --force-with-lease", true, func(ctx context.Context) (string, error) {
			return m.runner.ForcePushOutput(ctx)
		})
	}
	return m, nil
}

// handleConflictsLoaded stores conflict rows and redraws the conflict screen.
func (m Model) handleConflictsLoaded(msg conflictsLoadedMsg) Model {
	logging.Info("tui", "handleConflictsLoaded", "conflicts_loaded", logging.F("conflicts", len(msg.conflicts)), logging.F("error", msg.err))
	m.info = msg.info
	m.files = msg.files
	m.conflicts = msg.conflicts
	m.mode = "conflicts"
	m.reconcileFileCursor()
	m.reconcileConflictCursor()
	m.err = msg.err
	m.notice = ""

	review := msg.review
	if len(m.conflicts) > 0 {
		review = highlightPreview(m.conflicts[m.conflictCursor], review)
	}
	m.review.SetContent(m.conflictsView(msg.markerReport, review))
	m.review.GotoTop()
	return m
}

// handleStashesLoaded stores stash rows and redraws the stash screen.
func (m Model) handleStashesLoaded(msg stashesLoadedMsg) Model {
	logging.Info("tui", "handleStashesLoaded", "stashes_loaded", logging.F("stashes", len(msg.stashes)), logging.F("error", msg.err))
	m.info = msg.info
	m.files = msg.files
	m.stashes = msg.stashes
	m.mode = "stashes"
	m.reconcileFileCursor()
	m.reconcileStashCursor()
	m.err = msg.err
	m.notice = msg.notice

	m.review.SetContent(m.stashesView(msg.review))
	m.review.GotoTop()
	return m
}

// handleSquashLoaded stores the commits ahead of the default branch and draws
// the squash picker. The user chooses the base explicitly.
func (m Model) handleSquashLoaded(msg squashLoadedMsg) Model {
	logging.Info("tui", "handleSquashLoaded", "squash_loaded", logging.F("commits", len(msg.commits)), logging.F("error", msg.err))
	m.info = msg.info
	m.files = msg.files
	m.commits = msg.commits
	m.reconcileFileCursor()
	m.err = msg.err

	if msg.err != nil {
		// Fall back to the review screen so the reason is visible in the output bar.
		m.mode = "review"
		m.notice = ""
		m.gitOutput = msg.err.Error()
		m.review.SetContent(m.squashUnavailableView(msg.err))
		m.review.GotoTop()
		return m
	}

	m.mode = "squash"
	m.notice = ""
	// Every commit starts as "keep"; the user explicitly picks the base.
	m.squashMark = make([]bool, len(m.commits))
	m.squashBase = -1
	m.squashCursor = 0
	m.reconcileSquashCursor()
	m.review.SetContent(m.squashView())
	m.review.GotoTop()
	return m
}

// handleCommitMessageGenerated puts Codex's subject into the commit input.
func (m Model) handleCommitMessageGenerated(msg commitMessageGeneratedMsg) Model {
	m.loading = false
	m.err = msg.err
	m.mode = "commit"
	m.commit.Focus()
	if msg.err != nil {
		logging.Error("tui", "handleCommitMessageGenerated", "commit_message_generate_failed", logging.F("error", msg.err))
		m.notice = ""
		m.gitOutput = msg.err.Error()
		return m
	}
	logging.Info("tui", "handleCommitMessageGenerated", "commit_message_generate_complete", logging.F("message_bytes", len(msg.message)))
	m.commit.SetValue(msg.message)
	m.notice = "Generated commit message"
	m.gitOutput = msg.message
	return m
}

// Commit Screen Keys

// updateCommit handles typing a commit message and pressing enter to commit.
func (m Model) updateCommit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.mode = "review"
		m.commit.Blur()
		m.notice = "Commit cancelled"
		logging.Info("tui", "updateCommit", "commit_cancelled")
		return m, nil
	case "ctrl+g":
		logging.Info("tui", "updateCommit", "commit_message_generate_requested")
		return m.generateCommitMessageAction()
	case "enter":
		message := strings.TrimSpace(m.commit.Value())
		if message == "" {
			m.err = fmt.Errorf("commit message is required")
			logging.Warn("tui", "updateCommit", "commit_message_missing")
			return m, nil
		}
		m.commit.Blur()
		m.mode = "review"
		logging.Info("tui", "updateCommit", "commit_requested", logging.F("message_bytes", len(message)))
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

// Conflict Screen Keys

// updateConflicts handles unmerged files and common rebase conflict commands.
func (m Model) updateConflicts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "review"
		return m, loadReview(m.runner, m.selectedPath())
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		return m, loadConflicts(m.runner, m.conflictCursor)
	case "down":
		m = m.moveConflictCursor(1)
		return m, loadConflicts(m.runner, m.conflictCursor)
	case "up":
		m = m.moveConflictCursor(-1)
		return m, loadConflicts(m.runner, m.conflictCursor)
	case "enter":
		if len(m.conflicts) == 0 {
			return m.withNotice("No conflict files"), nil
		}
		return m.openConflictInEditor()
	case "m":
		if len(m.conflicts) == 0 {
			return m.withNotice("No conflict files"), nil
		}
		path := m.conflicts[m.conflictCursor]
		return m.action("Marked "+path+" resolved", false, func(ctx context.Context) (string, error) {
			return m.runner.MarkResolvedOutput(ctx, path)
		})
	case "c":
		return m.action("Rebase continued", false, func(ctx context.Context) (string, error) {
			return m.runner.RebaseContinueOutput(ctx)
		})
	case "a":
		return m.action("Rebase aborted", false, func(ctx context.Context) (string, error) {
			return m.runner.RebaseAbortOutput(ctx)
		})
	case "s":
		return m.action("Rebase skipped current patch", false, func(ctx context.Context) (string, error) {
			return m.runner.RebaseSkipOutput(ctx)
		})
	default:
		return m.updateViewportKey(msg)
	}
}

func (m Model) openConflictInEditor() (tea.Model, tea.Cmd) {
	if len(m.conflicts) == 0 {
		return m.withNotice("No conflict files"), nil
	}
	m.target = m.conflicts[m.conflictCursor]
	return m.openSelectedInEditor()
}

// Stash Screen Keys

// updateStashes handles the stash stack and selected stash actions.
func (m Model) updateStashes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "review"
		return m, loadReview(m.runner, m.selectedPath())
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		return m, loadStashes(m.runner, m.stashCursor)
	case "down":
		m = m.moveStashCursor(1)
		return m, loadStashes(m.runner, m.stashCursor)
	case "up":
		m = m.moveStashCursor(-1)
		return m, loadStashes(m.runner, m.stashCursor)
	case "n":
		return m.action("Stashed current changes", false, func(ctx context.Context) (string, error) {
			return m.runner.StashPushOutput(ctx)
		})
	case "a":
		stash, ok := m.selectedStash()
		if !ok {
			return m.withNotice("No stash selected"), nil
		}
		return m.action("Applied "+stash.Ref, false, func(ctx context.Context) (string, error) {
			return m.runner.StashApplyOutput(ctx, stash.Ref)
		})
	case "p":
		stash, ok := m.selectedStash()
		if !ok {
			return m.withNotice("No stash selected"), nil
		}
		return m.action("Popped "+stash.Ref, false, func(ctx context.Context) (string, error) {
			return m.runner.StashPopOutput(ctx, stash.Ref)
		})
	case "D":
		stash, ok := m.selectedStash()
		if !ok {
			return m.withNotice("No stash selected"), nil
		}
		return m.action("Dropped "+stash.Ref, false, func(ctx context.Context) (string, error) {
			return m.runner.StashDropOutput(ctx, stash.Ref)
		})
	default:
		return m.updateViewportKey(msg)
	}
}

func (m Model) selectedStash() (git.Stash, bool) {
	if len(m.stashes) == 0 {
		return git.Stash{}, false
	}
	return m.stashes[m.stashCursor], true
}

// Squash Screen Keys

// updateSquash drives the in-TUI squash picker. Each commit is explicitly marked
// keep, squash, or base.
func (m Model) updateSquash(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "review"
		return m, loadReview(m.runner, m.selectedPath())
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		return m, loadSquash(m.runner, m.config.DefaultBranch)
	case "down":
		m = m.moveSquashCursor(1)
		m.review.SetContent(m.squashView())
		return m, nil
	case "up":
		m = m.moveSquashCursor(-1)
		m.review.SetContent(m.squashView())
		return m, nil
	case "G":
		m = m.moveSquashCursor(len(m.commits))
		m.review.SetContent(m.squashView())
		return m, nil
	case "g":
		m = m.moveSquashCursor(-len(m.commits))
		m.review.SetContent(m.squashView())
		return m, nil
	case "S", "s", " ":
		return m.markSquashCommit()
	case "K", "k":
		return m.keepSquashCommit()
	case "B":
		return m.markSquashBase()
	case "a":
		return m.markAllSquash()
	case "enter":
		return m.applySquashPlan()
	}
	return m, nil
}

// markSquashCommit marks the commit under the cursor to be folded into whichever
// base the user chooses.
func (m Model) markSquashCommit() (tea.Model, tea.Cmd) {
	if len(m.commits) == 0 {
		return m.withNotice("No commits to squash"), nil
	}
	if m.squashCursor == m.squashBase {
		m.squashBase = -1
	}
	m.squashMark[m.squashCursor] = true
	m.notice = ""
	m.err = nil
	m.review.SetContent(m.squashView())
	return m, nil
}

// keepSquashCommit marks the commit under the cursor to remain separate.
func (m Model) keepSquashCommit() (tea.Model, tea.Cmd) {
	if len(m.commits) == 0 {
		return m.withNotice("No commits to keep"), nil
	}
	if m.squashCursor == m.squashBase {
		m.squashBase = -1
	}
	m.squashMark[m.squashCursor] = false
	m.notice = ""
	m.err = nil
	m.review.SetContent(m.squashView())
	return m, nil
}

// markSquashBase makes the commit under the cursor the target for squashed
// commits. A base commit is always kept.
func (m Model) markSquashBase() (tea.Model, tea.Cmd) {
	if len(m.commits) == 0 {
		return m.withNotice("No commits to use as base"), nil
	}
	m.squashBase = m.squashCursor
	m.squashMark[m.squashBase] = false
	m.notice = ""
	m.err = nil
	m.review.SetContent(m.squashView())
	return m, nil
}

// markAllSquash selects every commit so the whole branch collapses into one.
func (m Model) markAllSquash() (tea.Model, tea.Cmd) {
	if len(m.commits) < 2 {
		return m.withNotice("Need at least two commits to squash"), nil
	}
	for i := range m.squashMark {
		m.squashMark[i] = true
	}
	if m.squashBase >= 0 && m.squashBase < len(m.squashMark) {
		m.squashMark[m.squashBase] = false
	}
	m.notice = ""
	m.err = nil
	m.review.SetContent(m.squashView())
	return m, nil
}

// applySquashPlan runs the selected squash plan against the default branch.
func (m Model) applySquashPlan() (tea.Model, tea.Cmd) {
	if len(m.commits) < 2 {
		return m.withNotice("Need at least two commits to squash"), nil
	}
	if !m.squashBaseSelected() {
		return m.withNotice("Choose a base commit with B before applying the squash"), nil
	}
	folds := m.squashFoldCount()
	if folds == 0 {
		return m.withNotice("Mark at least one commit with S to squash into the base"), nil
	}

	actions := m.squashActions()
	remaining := len(m.commits) - folds
	base := m.config.DefaultBranch
	m.mode = "review"
	m.notice = ""
	m.err = nil
	logging.Info("tui", "applySquashPlan", "squash_requested", logging.F("folds", folds), logging.F("remaining", remaining))
	return m.action(fmt.Sprintf("Squashed down to %d commits", remaining), true, func(ctx context.Context) (string, error) {
		return m.runner.ApplySquash(ctx, base, actions)
	})
}

// squashActions builds the newest-first selection slice for the git layer.
func (m Model) squashActions() []git.SquashAction {
	actions := make([]git.SquashAction, len(m.commits))
	baseSelected := m.squashBaseSelected()
	for i, commit := range m.commits {
		actions[i] = git.SquashAction{Hash: commit.Hash, Squash: m.squashMark[i], Base: baseSelected && i == m.squashBase}
	}
	return actions
}

// squashFoldCount reports how many commits would be merged away by the plan.
func (m Model) squashFoldCount() int {
	if !m.squashBaseSelected() {
		count := 0
		for _, marked := range m.squashMark {
			if marked {
				count++
			}
		}
		return count
	}
	count := 0
	for _, line := range git.ResolveSquashPlan(m.squashActions()) {
		if line.Action == "squash" {
			count++
		}
	}
	return count
}

func (m Model) squashBaseSelected() bool {
	return m.squashBase >= 0 && m.squashBase < len(m.commits)
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
				return out, remoteErr
			}
			return out, localErr
		})
	}
	return m.action("Deleted "+branch+" (local)", true, func(ctx context.Context) (string, error) {
		return m.runner.DeleteBranchOutput(ctx, branch)
	})
}

// Discard File Prompt Keys

// updateDiscardFile confirms before removing the selected file's changes.
func (m Model) updateDiscardFile(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.mode = "review"
		m.notice = "Discard cancelled"
		return m, nil
	case "y":
		file, ok := m.selectedFile()
		if !ok {
			m.mode = "review"
			return m.withNotice("No file selected"), nil
		}
		return m.discardFile(file)
	}
	return m, nil
}

func (m Model) discardFile(file git.FileStatus) (tea.Model, tea.Cmd) {
	m.mode = "review"
	return m.action("Discarded "+file.DisplayPath(), true, func(ctx context.Context) (string, error) {
		return m.runner.DiscardOutput(ctx, file)
	})
}

func stageNotice(file git.FileStatus) string {
	switch {
	case file.Renamed():
		return "Staged rename " + file.DisplayPath()
	case file.Deleted():
		return "Staged removal of " + file.DisplayPath()
	default:
		return "Staged " + file.DisplayPath()
	}
}

func unstageNotice(file git.FileStatus) string {
	switch {
	case file.Renamed():
		return "Unstaged rename " + file.DisplayPath()
	case file.Deleted():
		return "Unstaged removal of " + file.DisplayPath()
	default:
		return "Unstaged " + file.DisplayPath()
	}
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

// updatePullRequest lets the user choose manual or generated PR creation.
func (m Model) updatePullRequest(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "review"
		m.notice = "Pull request cancelled"
		logging.Info("tui", "updatePullRequest", "pr_cancelled")
		return m, loadReview(m.runner, m.selectedPath())
	case "q", "ctrl+c":
		logging.Info("tui", "updatePullRequest", "quit", logging.F("key", msg.String()))
		return m, tea.Quit
	case "g":
		m.mode = "review"
		logging.Info("tui", "updatePullRequest", "generated_pr_requested")
		return m.pullRequestAction()
	case "m":
		m.mode = "review"
		logging.Info("tui", "updatePullRequest", "manual_pr_requested")
		return m.manualPullRequestAction()
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
