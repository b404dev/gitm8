# Codebase Guide

`gitm8` is a terminal UI first. It does not try to replace Git. It runs the
installed `git` and `gh` commands and gives them a cleaner screen.

## Upstream Docs Used By This App

These are the core library docs worth keeping open while reading the code:

- Bubble Tea tutorial: [Model, Init, Update, and View](https://pkg.go.dev/github.com/charmbracelet/bubbletea#section-readme)
- Bubble Tea API: [`tea.Model`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Model), [`tea.Msg`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Msg), [`tea.Cmd`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Cmd), [`tea.NewProgram`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#NewProgram)
- Bubble Tea messages/options used here: [`tea.KeyMsg`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#KeyMsg), [`tea.WindowSizeMsg`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#WindowSizeMsg), [`tea.Tick`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Tick), [`tea.Batch`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Batch), [`tea.ExecProcess`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#ExecProcess), [`tea.WithAltScreen`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#WithAltScreen)
- Bubbles components used here: [`viewport`](https://pkg.go.dev/github.com/charmbracelet/bubbles/viewport), [`textinput`](https://pkg.go.dev/github.com/charmbracelet/bubbles/textinput), [`spinner`](https://pkg.go.dev/github.com/charmbracelet/bubbles/spinner)
- Lip Gloss styling and layout: [`lipgloss.Style`](https://pkg.go.dev/github.com/charmbracelet/lipgloss#Style), [`lipgloss.JoinHorizontal`](https://pkg.go.dev/github.com/charmbracelet/lipgloss#JoinHorizontal), [`lipgloss.JoinVertical`](https://pkg.go.dev/github.com/charmbracelet/lipgloss#JoinVertical), [`lipgloss.Place`](https://pkg.go.dev/github.com/charmbracelet/lipgloss#Place)
- Go process execution: [`os/exec`](https://pkg.go.dev/os/exec)

## Layer Map

## Core Abstractions

These names show up everywhere:

### `tui.Model`

Defined in [`internal/tui/model.go`](../internal/tui/model.go).

`Model` is the full state of the terminal UI. If something can change on screen,
it probably lives on `Model`: selected file, current mode, repo info, loaded
files, branch list, spinner state, and the current output text.

Bubble Tea repeatedly passes `Model` through:

```text
Model.Update(...)
  -> returns changed Model
  -> Model.View() draws that changed state
```

### `git.Runner`

Defined in [`internal/git/git.go`](../internal/git/git.go).

`Runner` is the app's doorway to Git. The TUI should not build raw `git`
commands itself. It should call methods like:

```text
runner.StageOutput(...)
runner.SyncOutput(...)
runner.Branches(...)
```

Then `internal/git` decides the exact command to run.

### `config.Config`

Defined in [`internal/config/config.go`](../internal/config/config.go).

`Config` is loaded once at startup and stored on `Model`. It controls things
like theme, fetch-on-startup, commit graph display, and identity profiles.

### Custom Bubble Tea Messages

Defined in [`internal/tui/model.go`](../internal/tui/model.go).

These messages are how background work reports back to `Update`:

- `repoLoadedMsg`: repo info, changed files, and viewer text finished loading.
- `branchesLoadedMsg`: branch list finished loading.
- `gitActionFinishedMsg`: a Git command finished, with output or an error.

### Binary Startup

Start here when you want to understand how the program launches:

- [`main.go`](../main.go) checks that the user ran plain `gitm8`, loads config,
  optionally fetches on startup, and starts the UI with
  [`tea.NewProgram`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#NewProgram)
  and [`tea.WithAltScreen`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#WithAltScreen).
- [`internal/config/config.go`](../internal/config/config.go) reads defaults,
  dotfiles, environment variables, and identity profiles.
- [`internal/tui/model.go`](../internal/tui/model.go) creates the first UI state
  that satisfies Bubble Tea's [`tea.Model`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Model)
  interface.

The startup path is:

```text
main.go
  -> config.Load()
  -> git.NewRunner("")
  -> tui.New(runner, cfg)
  -> tea.NewProgram(...).Run()
```

Terms used below:

- `Model`: the struct that holds the current UI state. See Bubble Tea's
  [`Model`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Model).
- `Update`: the function Bubble Tea calls when something happens, such as a key
  press or a finished Git command. See the Bubble Tea
  [Update tutorial](https://pkg.go.dev/github.com/charmbracelet/bubbletea#section-readme).
- `View`: the function that turns the current state into terminal text. See the
  Bubble Tea [View tutorial](https://pkg.go.dev/github.com/charmbracelet/bubbletea#section-readme).
- `tea.Cmd`: a background job. In this app it usually loads Git data or runs a
  Git command, then sends the result back to `Update`. See Bubble Tea's
  [`Cmd`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Cmd).

### Configuration Layer

The config layer reads user settings and keeps them separate from UI state:

- [`internal/config/config.go`](../internal/config/config.go)
- [`internal/config/config_test.go`](../internal/config/config_test.go)

Read this layer when changing:

- environment variables such as `GITM8_THEME`
- `~/.gitm8rc` or `~/.gitm8/credentials` parsing
- profile parsing from `~/.gitm8/profiles`
- default behavior such as fetch-on-startup or commit graph display

### Git Layer

The Git layer is the only place that should run `git` or `gh`. The UI asks for
things like "stage this file" or "sync this branch"; this package decides the
exact command to run.

- Core types: [`internal/git/git.go`](../internal/git/git.go)
- Process execution: [`internal/git/exec.go`](../internal/git/exec.go)
- Basic commands: [`internal/git/commands.go`](../internal/git/commands.go)
- Staging and diffs: [`internal/git/changes.go`](../internal/git/changes.go)
- Branch workflows: [`internal/git/branch.go`](../internal/git/branch.go)
- Repository status/header data: [`internal/git/repo.go`](../internal/git/repo.go)
- File previews: [`internal/git/preview.go`](../internal/git/preview.go)
- Commit logs: [`internal/git/log.go`](../internal/git/log.go)
- Pull requests via `gh`: [`internal/git/pr.go`](../internal/git/pr.go)
- Sync workflow: [`internal/git/sync.go`](../internal/git/sync.go)
- Rebase commands: [`internal/git/rebase.go`](../internal/git/rebase.go)
- Remote URL helpers: [`internal/git/remote.go`](../internal/git/remote.go)
- Text formatting helpers: [`internal/git/text.go`](../internal/git/text.go)
- Tests: [`internal/git/git_test.go`](../internal/git/git_test.go)

Use this layer when changing what Git command runs. Do not put raw
`exec.Command` calls in the TUI. Add a `git.Runner` method instead.

### TUI State Layer

The Bubble Tea model is plain on purpose. State lives in one struct. The rest
of the files are split by job.

- Model and messages: [`internal/tui/model.go`](../internal/tui/model.go)
- Key handling and mode changes: [`internal/tui/update.go`](../internal/tui/update.go)
  uses Bubble Tea [`Msg`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Msg),
  [`KeyMsg`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#KeyMsg), and
  [`WindowSizeMsg`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#WindowSizeMsg).
- Git actions that run in the background: [`internal/tui/actions.go`](../internal/tui/actions.go)
  returns Bubble Tea [`Cmd`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Cmd)
  values and uses [`tea.Batch`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Batch)
  for spinner plus work.
- Data loading that runs in the background: [`internal/tui/load.go`](../internal/tui/load.go)
  returns custom messages back into `Update`; this follows Bubble Tea's
  [`Cmd`](https://pkg.go.dev/github.com/charmbracelet/bubbletea#Cmd) pattern.
- Cursor and list offset logic: [`internal/tui/cursor.go`](../internal/tui/cursor.go)

The core UI loop is:

```text
Model.Init()
  -> starts the first background job
  -> Model.Update(message from that job or from a key press)
  -> maybe starts another background job
  -> Model.View() draws the screen
```

Most user actions follow this path:

```text
key press
  -> updateKey
  -> updateDashboardKey or updateFocusedMode
  -> action(...) or load...
  -> git.Runner method
  -> result message: gitActionFinishedMsg, repoLoadedMsg, or branchesLoadedMsg
  -> handleGitActionFinished / handleRepoLoaded / handleBranchesLoaded
  -> View()
```

### TUI Rendering Layer

Rendering is separate from key handling:

- Full screen layout, picker views, help, and splash screen:
  [`internal/tui/view.go`](../internal/tui/view.go), built around Bubble Tea's
  `View` method and Lip Gloss layout helpers like
  [`JoinHorizontal`](https://pkg.go.dev/github.com/charmbracelet/lipgloss#JoinHorizontal),
  [`JoinVertical`](https://pkg.go.dev/github.com/charmbracelet/lipgloss#JoinVertical),
  and [`Place`](https://pkg.go.dev/github.com/charmbracelet/lipgloss#Place).
- Themes and styles: [`internal/tui/theme.go`](../internal/tui/theme.go), built
  around [`lipgloss.Style`](https://pkg.go.dev/github.com/charmbracelet/lipgloss#Style).
- Preview syntax highlighting: [`internal/tui/syntax.go`](../internal/tui/syntax.go)
- Text/layout helpers: [`internal/tui/text.go`](../internal/tui/text.go)
- Tests: [`internal/tui/model_test.go`](../internal/tui/model_test.go)

The Bubbles components are stored in `Model` and rendered from `View`:

- [`viewport.Model`](https://pkg.go.dev/github.com/charmbracelet/bubbles/viewport#Model)
  powers the scrollable review/preview/log/help panel.
- [`textinput.Model`](https://pkg.go.dev/github.com/charmbracelet/bubbles/textinput#Model)
  powers commit-message and branch-name inputs.
- [`spinner.Model`](https://pkg.go.dev/github.com/charmbracelet/bubbles/spinner#Model)
  powers the small "git working..." indicator.

Read these files when changing how something looks. Read `internal/git` when
changing what Git command runs.

### Installer And User Docs

- README: [`README.md`](../README.md)
- Abstractions guide: [`docs/ABSTRACTIONS.md`](ABSTRACTIONS.md)
- Flow examples: [`docs/FLOWS.md`](FLOWS.md)
- Man page: [`docs/man/gitm8.1`](man/gitm8.1)
- Installer: [`scripts/install.sh`](../scripts/install.sh)
- Example yazi config: [`configs/yazi/yazi.toml`](../configs/yazi/yazi.toml)
- Example yazi theme: [`configs/yazi/theme.toml`](../configs/yazi/theme.toml)

When user-visible behavior changes, update the README and man page together.

When the names feel unclear, read [`docs/ABSTRACTIONS.md`](ABSTRACTIONS.md).
When the control flow feels unclear, read [`docs/FLOWS.md`](FLOWS.md). It walks
through real keys like `s`, `x`, `b`, and `c`.

## Common Changes

### Add Or Change A Keybinding

1. Add the key handling in [`internal/tui/update.go`](../internal/tui/update.go).
   Normal dashboard keys and one-screen-only keys both live there.
2. Put Git operations in [`internal/git`](../internal/git), not directly in the
   TUI.
3. Update visible key references in [`internal/tui/view.go`](../internal/tui/view.go).
4. Update [`README.md`](../README.md) and [`docs/man/gitm8.1`](man/gitm8.1).

### Add A Git Operation

1. Add a method on `git.Runner` in the relevant [`internal/git`](../internal/git)
   file.
2. Return output when the UI should show command details.
3. Call the runner method from [`internal/tui/actions.go`](../internal/tui/actions.go)
   or the relevant mode handler.
4. Add a focused test if the change parses text, formats text, or recovers from
   a failed Git command.

### Change A View

1. Update [`internal/tui/view.go`](../internal/tui/view.go) for the main screen layout.
2. Update [`internal/tui/view.go`](../internal/tui/view.go) for picker content.
3. Update [`internal/tui/theme.go`](../internal/tui/theme.go) when changing
   colors or style variables.
4. Run the TUI in a real terminal after tests, because layout issues are easiest
   to see interactively.

## Verification

Use writable Go caches in this environment:

```sh
GOCACHE=/tmp/gitm8-go-build GOMODCACHE=/tmp/gitm8-go-mod go test ./...
GOCACHE=/tmp/gitm8-go-build GOMODCACHE=/tmp/gitm8-go-mod go build ./...
```
