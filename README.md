<img width="2560" height="1600" alt="screenshot-2026-06-13-09-52-28" src="https://github.com/user-attachments/assets/58c9fcd8-92e2-4109-9175-3347215e87b7" />

<img width="2560" height="1600" alt="screenshot-2026-06-13-09-50-21" src="https://github.com/user-attachments/assets/5dbb7424-e155-4eb5-a73b-56c9cb628e97" />

# gitm8

`gitm8` is a small terminal UI for common Git workflows. It uses the installed
`git` binary, so your existing Git configuration, SSH keys, credential helpers,
hooks, aliases, and signing behavior continue to work normally.

It provides one interface: an interactive dashboard for day-to-day Git work.

## Install

Recommended install:

```sh
curl -fsSL https://raw.githubusercontent.com/b404dev/gitm8/main/scripts/install.sh | bash
```

The script supports `apt`, `pacman`, and `brew`. It installs build/runtime
dependencies for you:

- Go, used to build `gitm8` from source
- `git`
- GitHub CLI (`gh`)

It then builds `gitm8`, installs it to `~/.local/bin`, and adds that directory
to your shell profile if needed.

Installer options:

```sh
GITM8_INSTALL_DIR="$HOME/bin"        # install somewhere else
GITM8_MAN_DIR="$HOME/.local/share/man/man1"
GITM8_SKIP_DEPS=1                    # skip package-manager dependency install
GITM8_SKIP_PATH_UPDATE=1             # do not edit shell profile
GITM8_SKIP_MAN_INSTALL=1             # do not install the man page
GITM8_REF=main                       # branch/tag to build
```

The installer also installs a manual page, so the command is available through
`man gitm8` when your man path includes `~/.local/share/man`.

If the repository is private, the raw script URL must be reachable from that
machine. Authenticate GitHub first, then use the Go install fallback if the raw
URL is not accessible:

```sh
gh auth login -h github.com
gh auth setup-git
```

Manual Go install:

If you already have Go installed, you can install directly with:

```sh
GOPRIVATE=github.com/b404dev/gitm8 go install github.com/b404dev/gitm8@latest
```

`go install` places the binary in `$(go env GOPATH)/bin`, usually `~/go/bin`.
It does not update your `PATH`. If `gitm8` is not found after install, add Go's
bin directory to your shell profile:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

If you do not have Go, install it first:

```sh
# Debian/Ubuntu
sudo apt-get update && sudo apt-get install -y golang-go

# Arch
sudo pacman -Sy --needed go

# macOS/Linux with Homebrew
brew install go
```

For local development:

```sh
go run .          # run the dashboard from source
go build -o gitm8 . && ./gitm8
```

## Dashboard

Run `gitm8` to open the dashboard.

For best results, launch `gitm8` from inside the Git repository you want to work
on:

```sh
cd path/to/project
gitm8
```

Most dashboard features expect the current directory to be part of a Git
worktree. If you open `gitm8` outside a repository, it opens the project picker
and lists repositories beneath `GITM8_WORKSPACE_DIR`. You can open an existing
repository, clone one from GitHub with `c`, or initialize one with `n`.

The layout is:

- top repository status bar
- git output bar
- changed-files panel on the left
- preview/review/log panel on the right
- footer keybinding bar

The dashboard opens on the first changed file's contents. Press `d` to toggle
that file's diff, or `0` for the repo-wide diff review. File previews use
lightweight, theme-aware syntax highlighting for common source and config files.
When a file is being viewed, press `enter` to open it in the configured editor.
Press `/` while viewing file contents to search and highlight matches in place,
then use `↑`/`↓` to move between matches. The output bar shows the search input,
match count, and search keys while the viewer keeps file context. Plain searches
match full words; use `*` or `?` wildcards to match partial words.
Press `x` to discard all changes to the selected file. This uses `git restore`
for tracked changes and `git clean -fd` for untracked files, and prompts first
when `GITM8_CONFIRM_DESTRUCTIVE_ACTIONS` is enabled.
The changed-files panel uses `DEL` for removed tracked paths and `REN` for
renames, shown as `old/path -> new/path`.

The top bar shows repository name, current branch, configured user, upstream,
ahead/behind counts, staged/unstaged counts, last fetch time, and active viewer
mode.

### Keys

| Key | Action |
| --- | --- |
| `↑` / `↓` | Move the file selection and preview it |
| `enter` | Open the viewed file in `GITM8_EDITOR` |
| `0` | Show the repo-wide code review |
| `d` | Toggle selected file between diff and contents |
| `/` | Search and highlight inside the viewed file contents |
| `x` | Discard all changes to selected file |
| `s` / `S` | Stage selected file / stage all changes |
| `u` / `U` | Unstage selected file / unstage all changes |
| `n` | Stash selected file |
| `c` | Commit staged changes |
| `f` | Fetch (`--all --prune`) |
| `p` / `P` | Pull (`--ff-only`) / push |
| `z` | In-TUI squash picker (mark commits, choose base, no editor) |
| `C` | Open conflict mode for unmerged files |
| `t` | Open stash panel |
| `b` | Open branch switcher |
| `l` | View recent commit logs |
| `r` | Open pull request options |
| `R` | Rebase current branch onto another branch |
| `i` | Switch Git identity profile |
| `h` | Open help |
| `o` | Expand/collapse the git output box |
| `tab` | Hide/show the footer key bar |
| `y` | Open `yazi` file manager, if installed |
| `w` | Open the workspace project picker |
| `j` / `k` | Scroll viewer line by line |
| `pgdn` / `pgup` (`ctrl+f` / `ctrl+b`) | Scroll viewer by a page |
| `g` / `G` | Jump viewer to top / bottom |
| `q` / `ctrl+c` | Quit |

### Branches

In the branch switcher (`b`):

| Key | Action |
| --- | --- |
| `↑` / `↓` | Choose a branch |
| `enter` | Switch to selected branch |
| `W` | Switch to selected branch and bring current changes via stash/pop |
| `n` | Create a branch; current changes remain in the working tree |
| `D` | Delete selected branch |
| `esc` | Return to review |

Deleting prompts whether to remove the branch locally only (`l`) or locally and
on the remote (`r`). Deletes are forced locally (`-D`) and stale remote-tracking
refs are pruned from the branch list.

The branch list shows local branches followed by remote-only branches.
Remote branches are de-duplicated and shown without their remote prefix, so
switching to `origin/feature` appears as `feature` and creates a local tracking
branch automatically.

### Rebase

In the rebase picker (`R`), use `↑`/`↓` to choose a target branch
and `enter` to rebase the current branch onto it. You cannot rebase onto the
current branch. If a rebase is already in progress, use `c` to continue, `a` to
abort, or `s` to skip the current patch.

If the rebase stops on conflicts, resolve them with normal Git commands outside
`gitm8`, such as:

```sh
git rebase --continue
git rebase --abort
```

### Conflicts

Press `C` to show files Git reports as unmerged. Use `↑`/`↓` to choose a file,
`enter` to open it in the configured editor, and `m` to stage it after you have
resolved the conflict. The selected file preview includes conflict marker line
numbers for `<<<<<<<`, `=======`, and `>>>>>>>`. Conflict mode also exposes
rebase controls: `c` continue, `a` abort, and `s` skip.

### Stashes

Press `n` from the main viewer to stash the selected file only.
Press `t` to open the stash panel. It lists `git stash list`, previews the
selected stash diff, and supports `n` to stash from there too, `a` to apply,
`p` to pop, and `D` to drop.

### Interactive Squash

Press `z` to open the in-TUI squash picker. It lists commits on the current
branch since the merge-base with `GITM8_DEFAULT_BRANCH`, newest first — no external editor is ever
opened. Move with `↑`/`↓`, press `S` to mark the highlighted commit for
squashing, `K` to keep it as a separate commit, and `B` to choose the
highlighted commit as the base. Press `a` to mark every commit for squashing,
then pick a base and press `enter` to apply. The base commit supplies the final
message, and the selected commits are combined into it. `gitm8` refuses to run
on the default branch, while detached, or with a dirty worktree.

Squashing rewrites local branch history, so it does not push for you. Press `P`
afterwards: if the remote rejects the push as a non-fast-forward, `gitm8` offers
to retry with `git push --force-with-lease`.

### Pull Requests

Press `r` in the dashboard to open pull request options for the
current branch. This workflow uses the GitHub CLI (`gh`):

```sh
gh auth login
```

Choose `g` to generate a title and description from the branch diff against the
configured default branch, then create the PR. This option is only shown when
the configured AI provider is installed. Choose `m` to write the PR yourself in
`gh pr create`. Set `GITM8_AI_PROVIDER=ollama` to use a local Ollama model via
`GITM8_OLLAMA_URL`. `gitm8` does not push while
creating a PR; push the branch first with `P` or normal Git. When the current
branch has no upstream, `P` uses `git push -u <remote> <branch>` when a remote
is configured. The generated path first tries to show an existing PR for the
current branch to avoid duplicates. If `gh` needs authentication, a pushed
branch, or more information, the error appears in the git output box.

### Git Identities

Press `i` to choose a configured identity profile. See [Profiles](#profiles).

Commit and new-branch inputs accept `enter` to confirm and `esc` to cancel.
In the commit input, press `ctrl+g` to ask the configured AI CLI to generate a
commit message from the currently staged changes and populate the input box.
That shortcut is hidden when the configured provider is not installed.

## Configuration

On first launch, if the configured workspace directory (initially `~/Github`)
does not exist, gitm8 opens a setup wizard. It collects the workspace path,
global Git name and email, initial branch name, and editor. Nothing is written
until the review screen is confirmed. After saving, the wizard can launch
`gh auth login` interactively. Declining is recorded in the gitm8 dotfile so
the wizard does not reappear.

Configuration is loaded from `~/.gitm8/.gitm8rc`, then `~/.gitm8/gitm8rc`,
then the legacy `~/.gitm8rc`, then `~/.gitm8/credentials`. Environment
variables override defaults.
On startup, `gitm8` creates `~/.gitm8/` and a default `~/.gitm8/.gitm8rc` if no
config file already exists. Existing config files are never replaced.
For first-run config generation, `gitm8` uses `codex` when it is installed,
falls back to `claude` when only Claude is installed, and otherwise writes a
clear AI unavailable comment. If an existing config selects a missing provider,
`gitm8` keeps that value and disables generated commit/PR text until the binary
is installed.

```sh
export GITM8_WORKSPACE_DIR="$HOME/Github"
export GITM8_FIRST_RUN_DISMISSED="false"
export GITM8_DEFAULT_BRANCH="main"
export GITM8_EDITOR="vim"
export GITM8_THEME="catppuccin"
export GITM8_CONFIRM_DESTRUCTIVE_ACTIONS="true"
export GITM8_FETCH_ON_STARTUP="false"
export GITM8_SHOW_COMMIT_GRAPH="true"
export GITM8_AI_PROVIDER="codex" # codex, claude, or ollama
export GITM8_OLLAMA_URL="http://localhost:11434"
export GITM8_LOG_ENABLED="true"
export GITM8_LOG_LEVEL="INFO" # DEBUG, INFO, WARN, ERROR
export GITM8_LOG_FILE="$HOME/.gitm8/gitm8.log"
```

An example config is available at [`configs/gitm8rc.example`](configs/gitm8rc.example).

Logs are written as plain text and do not use stdout or stderr during normal TUI
use. The log format is:

```text
YYYY-MM-DDTHH:MM:SSZ LEVEL component function event key=value
```

`GITM8_LOG_FILE` supports `~/` and `$HOME/` prefixes.
When the active log reaches 100 MB, `gitm8` compresses it and keeps up to five
backups beside the active log, such as `gitm8.log.1.gz`.

Secrets should normally stay in Git credential helpers, SSH agents, SSH keys,
environment variables, or the OS keychain.

## Themes

Set a theme with:

```sh
export GITM8_THEME="catppuccin"
```

Available themes:

```text
amber
catppuccin
catppuccin-frappe
catppuccin-latte
catppuccin-macchiato
catppuccin-mocha
cyan
default
forest
highvis
lime
midnight
mono
ocean
rose
steel
violet
```

`catppuccin` maps to `catppuccin-mocha`.

## Profiles

Define switchable Git identities in `~/.gitm8/profiles`, one per line:

```text
Work     = Ada Lovelace <ada@work.example>
Personal = Ada <ada@personal.example>
```

Blank lines and lines starting with `#` are ignored, as are malformed lines.

When you select a profile, `gitm8` runs:

```sh
git config user.name "<name>"
git config user.email "<email>"
```

This applies to the current repository only. Your global identity is left
untouched. The active identity is marked `(current)` in the list, and the top
bar updates after the switch completes.

Keep real names and addresses out of this repository; profiles live only on
your machine.

## yazi integration

Press `y` in the dashboard to launch the
[`yazi`](https://github.com/sxyazi/yazi) file manager, returning to `gitm8`
when you quit it.

Sample `yazi` configuration lives under [`configs/yazi/`](configs/yazi/).

## Codebase

For a maintainers' map of the layers and click-through source references, see
[`docs/CODEBASE.md`](docs/CODEBASE.md).

For explanations of the core names like `Model`, `Runner`, and custom messages,
see [`docs/ABSTRACTIONS.md`](docs/ABSTRACTIONS.md).

For concrete examples of how key presses move through the code, see
[`docs/FLOWS.md`](docs/FLOWS.md).

## License

MIT. See [LICENSE](LICENSE).
