# Changelog

All notable changes to `gitm8` are documented here.

This project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [3.1.0] - 2026-10-08

### Added

- Branded All Keys browser with searchable shortcuts, aligned action rows,
  visibility indicators, and selected-command details.
- Persistent main-footer customization through All Keys and
  `GITM8_KEY_REFERENCE`. Every label can be hidden without disabling its key.
- Color swatches in the theme picker and semantic status colors across themes.

### Changed

- Separated repository identity and Git status in the dashboard header.
- Improved file-list context, empty states, viewer headings, and reader titles.
- Lightweight footer shortcuts and clearer command-output status indicators.
- All Keys starts in browse mode; `/` explicitly starts search.

### Fixed

- Shortcut toggles follow the visible selection across pages and searches.
- Delayed search results no longer overwrite shortcut visibility changes.
- Help and Quit respect visibility settings; hiding every label stays saved.
- File and release mouse selection aligns with the updated layout.

## [2.1.0] - 2026-08-23

A smoother first-run and multi-project workflow for developers who launch
`gitm8` from their home directory or another location outside a worktree.

### Added

- First-run setup wizard when the configured workspace directory is missing.
- Reviewed setup for workspace location, global Git name/email, default branch,
  editor, and optional GitHub CLI authentication.
- Persistent dismissal of the first-run wizard.
- Workspace project picker when launching outside a Git repository.
- In-app switching between repositories with `w`, without restarting gitm8.
- Repository discovery beneath `GITM8_WORKSPACE_DIR`.
- GitHub repository cloning and local repository initialization from the picker.

### Changed

- Reworked the project picker to match the themed dashboard, with repository
  and workspace panels, selected-project details, quick actions, and responsive
  narrow-terminal rendering.
- Prevented stale repository loads from replacing the workspace picker with an
  empty `no repo` dashboard.

## [2.0.0] - 2026-08-14

A major release focused on rewriting history safely, AI-assisted Git text, and
working with individual files without leaving the dashboard.

### Added

- In-TUI squash picker on `z`. Mark commits, choose an explicit base, and
  `gitm8` builds the rebase todo and drives it non-interactively, so you never
  drop out to an external editor. Squashed commit messages are kept and merged.
- AI generation of commit subjects and pull request title/body from the staged
  diff, with three selectable providers via `GITM8_AI_PROVIDER`: Codex, Claude,
  and Ollama with local model detection.
- Pull request options menu on `r`, including a manual `gh pr create` path.
- JSON pull request draft format.
- Stash panel on `t`, plus per-file stashing on `n`.
- Conflict mode on `C` for unmerged files, with conflict marker reporting.
- Discard all changes to the selected file on `x`, using `git restore` for
  tracked changes and `git clean -fd` for untracked files.
- In-file search on `/`, highlighting matches in place with arrow-key
  navigation. Plain searches match full words; `*` and `?` wildcards match
  partial words.
- File filtering in the changed-files panel, with `DEL` and `REN` badges for
  removed and renamed paths.
- Configurable application file logging.
- First-run config setup, with an example at `configs/gitm8rc.example`.
- `matt_mode`, which always force pushes with a raw `--force`, with no safety
  net and no prompt. Off by default.
- Force push is offered after a non-fast-forward push rejection.
- Build script at `scripts/build.sh`.

### Changed

- Squash rebases from the branch merge base, so only the branch's own commits
  are in play.
- A squash base is now required rather than inferred.
- AI availability is detected and handled rather than assumed.
- Generated Git text avoids em dashes.
- Expanded man page and documentation.

### Removed

- The old sync flow (`internal/git/sync.go`).

## [1.0.0] - 2026-06-12

Initial public release. Terminal UI for common Git workflows, driving the
installed `git` binary so existing configuration, SSH keys, credential helpers,
hooks, aliases, and signing behavior continue to work.

[3.1.0]: https://github.com/b404dev/gitm8/compare/v3.0.0...v3.1.0
[2.1.0]: https://github.com/b404dev/gitm8/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/b404dev/gitm8/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/b404dev/gitm8/releases/tag/v1.0.0
