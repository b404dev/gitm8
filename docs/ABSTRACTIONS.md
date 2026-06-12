# Abstractions

This file explains the main names the code keeps using. Read this before
following the flow examples.

## App Shape

The app is split into four simple responsibilities:

- `main.go`: starts the app.
- `internal/config`: reads user settings.
- `internal/git`: runs `git` and `gh`.
- `internal/tui`: handles keys, state, and drawing.

The important rule is:

```text
TUI code asks for an app action.
Git code decides the exact git command.
View code draws whatever is currently in Model.
```

## `config.Config`

Defined in [`internal/config/config.go`](../internal/config/config.go).

`Config` is the user settings object. It is loaded once in `main.go`, then
stored on `tui.Model`.

It answers questions like:

- What theme should the UI use?
- Should gitm8 fetch on startup?
- Should commit logs show graph lines?
- Which Git identity profiles are available?

Important functions:

- `config.Load()`: builds the final config.
- `defaults()`: sets fallback values.
- `loadFile()`: reads `~/.gitm8rc` and `~/.gitm8/credentials`.
- `applyEnv()`: applies `GITM8_*` environment variables.
- `loadProfiles()`: reads `~/.gitm8/profiles`.

## `config.Profile`

Defined in [`internal/config/config.go`](../internal/config/config.go).

`Profile` is one selectable Git identity:

```text
Label: "Work"
Name:  "Ada Lovelace"
Email: "ada@work.example"
```

The TUI uses profiles on the identity screen opened with `i`.

## `git.Runner`

Defined in [`internal/git/git.go`](../internal/git/git.go).

`Runner` is the app's doorway to Git. It has one optional field:

- `Dir`: where Git commands should run. Empty means "use the current directory".

The TUI should call `Runner` methods instead of building commands directly.

Examples:

```text
m.runner.StageOutput(ctx, path)
m.runner.SyncOutput(ctx)
m.runner.Branches(ctx)
```

The actual process call is handled in [`internal/git/exec.go`](../internal/git/exec.go).

## `git.FileStatus`

Defined in [`internal/git/git.go`](../internal/git/git.go).

`FileStatus` is one row from `git status --porcelain`.

Fields:

- `Path`: file path.
- `Index`: staged status column from Git.
- `Worktree`: unstaged status column from Git.

Helpers:

- `Staged()`: true when the file has staged changes.
- `Unstaged()`: true when the file has unstaged or untracked changes.
- `Label()`: returns Git's short status label, such as `M ` or `??`.

The TUI uses this for the changed-files panel.

## `git.RepoInfo`

Defined in [`internal/git/git.go`](../internal/git/git.go).

`RepoInfo` is the small snapshot shown in the top status bar.

It contains:

- repo name
- current branch
- upstream branch
- configured Git user
- ahead/behind counts
- staged/unstaged counts
- last fetch time

It is built by `Runner.RepoInfo()` in [`internal/git/repo.go`](../internal/git/repo.go).

## `tui.Model`

Defined in [`internal/tui/model.go`](../internal/tui/model.go).

`Model` is the full UI state. Bubble Tea passes it around constantly:

```text
Model.Update(...)
  -> returns changed Model
  -> Model.View() draws that changed Model
```

Important fields:

- `runner`: the `git.Runner` used for Git commands.
- `config`: loaded app settings.
- `review`: scrollable viewport for preview/review/log/help text.
- `commit`: text input for commit messages.
- `branchInput`: text input for new branch names.
- `spinner`: loading indicator while Git work runs.
- `info`: current `git.RepoInfo`.
- `files`: changed files from Git status.
- `branches`: branch list for branch and rebase screens.
- `fileCursor`, `branchCursor`, `profileCursor`: selected row indexes.
- `target`: currently selected file path or `repo`.
- `mode`: current screen, such as `review`, `preview`, `branches`, or `commit`.
- `gitOutput`: latest command output shown in the header.

## TUI Modes

The TUI currently stores screen mode as a string in `Model.mode`.

Common modes:

- `review`: repo or file diff view.
- `preview`: file content preview.
- `logs`: commit log view.
- `branches`: branch picker.
- `rebase`: rebase target picker.
- `profiles`: Git identity picker.
- `commit`: commit message input.
- `new-branch`: new branch name input.
- `delete-branch`: branch delete prompt.
- `help`: help screen.

Keys first go through `updateKey()` in [`internal/tui/update.go`](../internal/tui/update.go).
That function sends keys to:

- `updateDashboardKey()` for normal dashboard keys.
- `updateFocusedMode()` for screens that take over input.

## Custom Bubble Tea Messages

Defined in [`internal/tui/model.go`](../internal/tui/model.go).

Bubble Tea commands return messages. gitm8 uses three custom message types:

### `repoLoadedMsg`

Means repo data finished loading.

Carries:

- `git.RepoInfo`
- changed files
- viewer text
- current target
- current mode
- error, if loading failed

Handled by `handleRepoLoaded()`.

### `branchesLoadedMsg`

Means branch data finished loading.

Carries:

- `git.RepoInfo`
- changed files
- branch names
- mode, either `branches` or `rebase`
- error, if loading failed

Handled by `handleBranchesLoaded()`.

### `gitActionFinishedMsg`

Means a Git command finished.

Carries:

- command output
- whether the screen should refresh
- error, if the command failed

Handled by `handleGitActionFinished()`.

## Bubbles Components Stored In `Model`

These come from Charm's Bubbles library:

- `viewport.Model`: scrollable text area for review, preview, logs, and help.
- `textinput.Model`: input field for commit messages and new branch names.
- `spinner.Model`: small loading indicator while Git work runs.

Docs:

- [`viewport`](https://pkg.go.dev/github.com/charmbracelet/bubbles/viewport)
- [`textinput`](https://pkg.go.dev/github.com/charmbracelet/bubbles/textinput)
- [`spinner`](https://pkg.go.dev/github.com/charmbracelet/bubbles/spinner)

## `palette`

Defined in [`internal/tui/theme.go`](../internal/tui/theme.go).

`palette` is the internal color set for one theme. `applyTheme()` copies a
palette into the shared Lip Gloss styles used by the whole TUI.

The shared styles are:

- `titleStyle`
- `errorStyle`
- `mutedStyle`
- `keyStyle`
- `panelStyle`
- `activeStyle`
- syntax highlight styles

Docs:

- [`lipgloss.Style`](https://pkg.go.dev/github.com/charmbracelet/lipgloss#Style)

## `splashTickMsg`

Defined in [`internal/tui/view.go`](../internal/tui/view.go).

`splashTickMsg` is a tiny message sent by `tea.Tick()` while the splash screen
is animating. Each tick advances the splash frame. When the splash finishes,
the app loads the first repo view.

Docs:

- [`tea.Tick`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Tick)
