package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/git"
)

// loadDefault opens the first changed file. If there are no changes, it shows
// the whole-repo review screen instead.
func loadDefault(runner git.Runner) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		info, infoErr := runner.RepoInfo(ctx)
		files, filesErr := runner.FileStatuses(ctx)
		if len(files) == 0 {
			review, reviewErr := runner.Review(ctx)
			return repoLoadedMsg{
				info:   info,
				files:  files,
				review: review,
				target: "repo",
				mode:   "review",
				err:    firstErr(infoErr, filesErr, reviewErr),
			}
		}

		path := files[0].Path
		preview, previewErr := runner.Preview(ctx, path)
		return repoLoadedMsg{
			info:   info,
			files:  files,
			review: preview,
			target: path,
			mode:   "preview",
			err:    firstErr(infoErr, filesErr, previewErr),
		}
	}
}

// loadReview loads staged and unstaged diffs for the main viewer.
func loadReview(runner git.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		info, infoErr := runner.RepoInfo(ctx)
		files, filesErr := runner.FileStatuses(ctx)
		review, reviewErr := runner.Review(ctx, optionalPath(path)...)

		target := path
		if target == "" {
			target = "repo"
		}
		return repoLoadedMsg{
			info:   info,
			files:  files,
			review: review,
			target: target,
			mode:   "review",
			err:    firstErr(infoErr, filesErr, reviewErr),
		}
	}
}

// loadPreview loads readable content for one selected file or directory.
func loadPreview(runner git.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		info, infoErr := runner.RepoInfo(ctx)
		files, filesErr := runner.FileStatuses(ctx)
		review, previewErr := runner.Preview(ctx, path)

		target := path
		if target == "" {
			target = "repo"
		}
		return repoLoadedMsg{
			info:   info,
			files:  files,
			review: review,
			target: target,
			mode:   "preview",
			err:    firstErr(infoErr, filesErr, previewErr),
		}
	}
}

// loadLog loads recent commits for the log viewer.
func loadLog(runner git.Runner, graph bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		info, infoErr := runner.RepoInfo(ctx)
		files, filesErr := runner.FileStatuses(ctx)
		logs, logErr := runner.Log(ctx, 30, graph)
		return repoLoadedMsg{
			info:   info,
			files:  files,
			review: logs,
			target: "repo",
			mode:   "logs",
			err:    firstErr(infoErr, filesErr, logErr),
		}
	}
}

// loadBranches loads branch names for the branch switcher.
func loadBranches(runner git.Runner) tea.Cmd {
	return loadBranchList(runner, "branches")
}

// loadRebase loads branch names for choosing a rebase target.
func loadRebase(runner git.Runner) tea.Cmd {
	return loadBranchList(runner, "rebase")
}

// loadConflicts loads unmerged files and previews the selected conflict file.
func loadConflicts(runner git.Runner, cursor int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		info, infoErr := runner.RepoInfo(ctx)
		files, filesErr := runner.FileStatuses(ctx)
		conflicts, conflictsErr := runner.ConflictFiles(ctx)

		review := "No conflict files found.\n"
		markerReport := ""
		var previewErr error
		if len(conflicts) > 0 {
			cursor = clamp(cursor, 0, len(conflicts)-1)
			markerReport = runner.ConflictMarkerReport(conflicts[cursor])
			review, previewErr = runner.Preview(ctx, conflicts[cursor])
		}
		return conflictsLoadedMsg{
			info:         info,
			files:        files,
			conflicts:    conflicts,
			markerReport: markerReport,
			review:       review,
			err:          firstErr(infoErr, filesErr, conflictsErr, previewErr),
		}
	}
}

// loadStashes loads the stash stack and previews the selected stash diff.
func loadStashes(runner git.Runner, cursor int) tea.Cmd {
	return loadStashesWithNotice(runner, cursor, "")
}

func loadStashesWithNotice(runner git.Runner, cursor int, notice string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		info, infoErr := runner.RepoInfo(ctx)
		files, filesErr := runner.FileStatuses(ctx)
		stashes, stashesErr := runner.Stashes(ctx)

		review := "No stashes found.\n"
		var diffErr error
		if len(stashes) > 0 {
			cursor = clamp(cursor, 0, len(stashes)-1)
			review, diffErr = runner.StashDiff(ctx, stashes[cursor].Ref)
		}
		return stashesLoadedMsg{
			info:    info,
			files:   files,
			stashes: stashes,
			review:  review,
			notice:  notice,
			err:     firstErr(infoErr, filesErr, stashesErr, diffErr),
		}
	}
}

// loadSquash loads the commits ahead of the default branch for the squash picker.
func loadSquash(runner git.Runner, base string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		info, infoErr := runner.RepoInfo(ctx)
		files, filesErr := runner.FileStatuses(ctx)
		commits, commitsErr := runner.SquashCandidates(ctx, base)
		return squashLoadedMsg{
			info:    info,
			files:   files,
			commits: commits,
			err:     firstErr(infoErr, filesErr, commitsErr),
		}
	}
}

// loadBranchList does the shared branch-loading work for switch and rebase screens.
func loadBranchList(runner git.Runner, mode string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		info, infoErr := runner.RepoInfo(ctx)
		files, filesErr := runner.FileStatuses(ctx)
		branches, branchesErr := runner.Branches(ctx)
		return branchesLoadedMsg{
			info:     info,
			files:    files,
			branches: branches,
			mode:     mode,
			err:      firstErr(infoErr, filesErr, branchesErr),
		}
	}
}

// loadCurrent reloads the current screen after an action changes the repo.
func loadCurrent(runner git.Runner, path string, mode string, graph bool) tea.Cmd {
	return loadCurrentWithNotice(runner, path, mode, graph, "")
}

func loadCurrentWithNotice(runner git.Runner, path string, mode string, graph bool, notice string) tea.Cmd {
	if mode == "branches" {
		return loadBranches(runner)
	}
	if mode == "rebase" {
		return loadRebase(runner)
	}
	if mode == "conflicts" {
		return loadConflicts(runner, 0)
	}
	if mode == "stashes" {
		return loadStashes(runner, 0)
	}
	if mode == "squash" {
		return loadReview(runner, path)
	}
	if mode == "logs" {
		return loadLog(runner, graph)
	}
	if mode == "preview" && path != "" {
		return loadPreviewWithNotice(runner, path, notice)
	}
	return loadReviewWithNotice(runner, path, notice)
}

func loadReviewWithNotice(runner git.Runner, path string, notice string) tea.Cmd {
	return func() tea.Msg {
		msg := loadReview(runner, path)().(repoLoadedMsg)
		msg.notice = notice
		return msg
	}
}

func loadPreviewWithNotice(runner git.Runner, path string, notice string) tea.Cmd {
	return func() tea.Msg {
		msg := loadPreview(runner, path)().(repoLoadedMsg)
		msg.notice = notice
		return msg
	}
}
