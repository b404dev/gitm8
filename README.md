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
- optional `yazi` support

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
worktree. If you open `gitm8` outside a repository, Git commands will fail until
you `cd` into a project.

The layout is:

- top repository status bar
- git output bar
- changed-files panel on the left
- preview/review/log panel on the right
- footer keybinding bar

The dashboard opens on the first changed file's contents. Press `d` to toggle
that file's diff, or `0` for the repo-wide diff review. File previews use
lightweight, theme-aware syntax highlighting for common source and config files.

The top bar shows repository name, current branch, configured user, upstream,
ahead/behind counts, staged/unstaged counts, last fetch time, and active viewer
mode.

### Keys

| Key | Action |
| --- | --- |
| `↑` / `↓` | Move the file selection and preview it |
| `enter` | Preview the selected file |
| `0` | Show the repo-wide code review |
| `d` | Toggle selected file between diff and contents |
| `s` / `S` | Stage selected file / stage all changes |
| `u` / `U` | Unstage selected file / unstage all changes |
| `c` | Commit staged changes |
| `f` | Fetch (`--all --prune`) |
| `p` / `P` | Pull (`--ff-only`) / push |
| `x` | Sync: pull with rebase, then push |
| `b` | Open branch switcher |
| `l` | View recent commit logs |
| `r` / `ctrl+p` | Open pull request options |
| `R` / `ctrl+r` | Rebase current branch onto another branch |
| `i` | Switch Git identity profile |
| `h` | Open help |
| `o` | Expand/collapse the git output box |
| `tab` | Hide/show the footer key bar |
| `y` | Open `yazi` file manager, if installed |
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

In the rebase picker (`R` or `ctrl+r`), use `↑`/`↓` to choose a target branch
and `enter` to rebase the current branch onto it. You cannot rebase onto the
current branch. If a rebase is already in progress, use `c` to continue, `a` to
abort, or `s` to skip the current patch.

If the rebase stops on conflicts, resolve them with normal Git commands outside
`gitm8`, such as:

```sh
git rebase --continue
git rebase --abort
```

### Pull Requests

Press `r` or `ctrl+p` in the dashboard to open pull request options for the
current branch. This workflow uses the GitHub CLI (`gh`):

```sh
gh auth login
```

Choose `g` to generate a title and description from the branch diff against the
configured default branch, then create the PR. Choose `m` to write the PR
yourself in `gh pr create`. The generated path first tries to show an existing
PR for the current branch to avoid duplicates. If `gh` needs authentication, a
pushed branch, or more information, the error appears in the git output box.

### Git Identities

Press `i` to choose a configured identity profile. See [Profiles](#profiles).

Commit and new-branch inputs accept `enter` to confirm and `esc` to cancel.
In the commit input, press `ctrl+g` to ask the local `codex` CLI to generate a
commit message from the currently staged changes and populate the input box.
Set `GITM8_AI_PROVIDER=claude` to use the local `claude` CLI instead.

## Configuration

Configuration is loaded from `~/.gitm8rc`, then `~/.gitm8/credentials`.
Environment variables override defaults.

```sh
export GITM8_DEFAULT_BRANCH="main"
export GITM8_EDITOR="vim"
export GITM8_THEME="catppuccin"
export GITM8_CONFIRM_DESTRUCTIVE_ACTIONS="true"
export GITM8_FETCH_ON_STARTUP="false"
export GITM8_SHOW_COMMIT_GRAPH="true"
export GITM8_AI_PROVIDER="codex" # codex or claude
```

An example config is available at [`configs/gitm8rc.example`](configs/gitm8rc.example).

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
