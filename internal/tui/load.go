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
	if mode == "branches" {
		return loadBranches(runner)
	}
	if mode == "rebase" {
		return loadRebase(runner)
	}
	if mode == "logs" {
		return loadLog(runner, graph)
	}
	if mode == "preview" && path != "" {
		return loadPreview(runner, path)
	}
	return loadReview(runner, path)
}
