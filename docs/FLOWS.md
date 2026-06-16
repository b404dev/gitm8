# TUI Flow Examples

This file shows how the app hangs together for real key presses.

For the common names used here, read [`docs/ABSTRACTIONS.md`](ABSTRACTIONS.md).

The basic rule is:

```text
key press
  -> internal/tui/update.go decides what the key means
  -> internal/tui/actions.go or internal/tui/load.go starts background work
  -> internal/git runs git or gh
  -> a message comes back to Update
  -> internal/tui/view.go draws the new screen
```

Names that appear in most flows:

- `Model`: the current UI state.
- `Runner`: the Git command wrapper stored on `Model` as `m.runner`.
- `Config`: the loaded user settings stored on `Model` as `m.config`.
- `repoLoadedMsg`, `branchesLoadedMsg`, `gitActionFinishedMsg`: messages sent
  back to `Update` when background work finishes.

## Start The App

```text
main.go
  -> config.Load()
  -> git.NewRunner("")
  -> tui.New(runner, cfg)
  -> tea.NewProgram(...).Run()
```

Important files:

- [`main.go`](../main.go)
- [`internal/config/config.go`](../internal/config/config.go)
- [`internal/tui/model.go`](../internal/tui/model.go)
- [`internal/tui/view.go`](../internal/tui/view.go)

What happens after startup:

```text
Model.Init()
  -> tickSplash()
  -> updateSplash()
  -> loadDefault()
  -> repoLoadedMsg
  -> handleRepoLoaded()
  -> View()
```

## Press `s` To Stage One File

```text
key: s
  -> update.go: updateKey()
  -> update.go: updateDashboardKey()
  -> selectedFile()
  -> FileStatus.GitPaths()
  -> action(...)
  -> git.Runner.StageOutput()
  -> git add -- <path...>
  -> gitActionFinishedMsg
  -> handleGitActionFinished()
  -> loadCurrent()
  -> repoLoadedMsg
  -> handleRepoLoaded()
  -> View()
```

Files to read:

- [`internal/tui/update.go`](../internal/tui/update.go)
- [`internal/tui/actions.go`](../internal/tui/actions.go)
- [`internal/git/changes.go`](../internal/git/changes.go)
- [`internal/tui/load.go`](../internal/tui/load.go)
- [`internal/tui/view.go`](../internal/tui/view.go)

`FileStatus.GitPaths()` matters for renames because Git needs both old and new
paths for some operations. The UI still shows `DisplayPath()` so users see
`old/path -> new/path`.

## Press `b` To Switch Branches

```text
key: b
  -> update.go: updateKey()
  -> update.go: updateDashboardKey()
  -> loadBranches()
  -> git.Runner.Branches()
  -> branchesLoadedMsg
  -> handleBranchesLoaded()
  -> branchesView()
  -> View()
```

Then pressing `enter` inside the branch picker:

```text
key: enter
  -> update.go: updateKey()
  -> update.go: updateFocusedMode()
  -> update.go: updateBranches()
  -> action(...)
  -> git.Runner.SwitchBranchOutput()
  -> git switch <branch>
  -> gitActionFinishedMsg
  -> handleGitActionFinished()
  -> loadCurrent()
  -> View()
```

Files to read:

- [`internal/tui/update.go`](../internal/tui/update.go)
- [`internal/tui/load.go`](../internal/tui/load.go)
- [`internal/git/branch.go`](../internal/git/branch.go)
- [`internal/tui/view.go`](../internal/tui/view.go)

## Press `c` To Commit

```text
key: c
  -> update.go: updateKey()
  -> update.go: updateDashboardKey()
  -> mode becomes "commit"
  -> header() shows the commit text input
```

Then typing a message and pressing `enter`:

```text
key: enter
  -> update.go: updateKey()
  -> update.go: updateFocusedMode()
  -> update.go: updateCommit()
  -> action(...)
  -> git.Runner.CommitOutput()
  -> git commit -m <message>
  -> gitActionFinishedMsg
  -> handleGitActionFinished()
  -> loadCurrent()
  -> View()
```

Files to read:

- [`internal/tui/update.go`](../internal/tui/update.go)
- [`internal/tui/view.go`](../internal/tui/view.go)
- [`internal/git/commands.go`](../internal/git/commands.go)

## Press `r` To Create Or Show A Pull Request

```text
key: r
  -> update.go: updateKey()
  -> update.go: updateDashboardKey()
  -> pull request options view
```

Then pressing `g` in the pull request options:

```text
key: g
  -> actions.go: pullRequestAction()
  -> git.Runner.PullRequestOutput()
  -> require current branch to already be pushed
  -> gh pr view, or generate title/body and gh pr create
  -> gitActionFinishedMsg
  -> handleGitActionFinished()
  -> View()
```

Then pressing `m` in the pull request options:

```text
key: m
  -> actions.go: manualPullRequestAction()
  -> gh pr create
  -> gitActionFinishedMsg
  -> handleGitActionFinished()
  -> View()
```

Files to read:

- [`internal/tui/update.go`](../internal/tui/update.go)
- [`internal/tui/actions.go`](../internal/tui/actions.go)
- [`internal/git/pr.go`](../internal/git/pr.go)

## Message Names

The app uses a few custom Bubble Tea messages:

- `repoLoadedMsg`: repo info, changed files, and viewer text finished loading.
- `branchesLoadedMsg`: branch list finished loading.
- `gitActionFinishedMsg`: a Git command finished, with output or an error.

These are defined in [`internal/tui/model.go`](../internal/tui/model.go) and
handled in [`internal/tui/update.go`](../internal/tui/update.go).
