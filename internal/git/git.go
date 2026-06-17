package git

import (
	"strings"
	"time"
)

// Runner is the app's doorway to the installed git binary. The TUI calls
// Runner methods instead of building raw git commands itself.
type Runner struct {
	Dir string
}

// FileStatus is one changed-file row from `git status --porcelain`.
type FileStatus struct {
	Path      string
	OldPath   string
	Index     byte
	Worktree  byte
	Directory bool
}

// Commit is one commit row used by the in-TUI squash workflow.
type Commit struct {
	Hash    string
	Subject string
}

// Stash is one row from `git stash list`.
type Stash struct {
	Ref     string
	Subject string
}

// Staged reports whether Git says this file has staged changes.
func (f FileStatus) Staged() bool {
	return f.Index != ' ' && f.Index != '?'
}

// Unstaged reports whether Git says this file has unstaged or untracked changes.
func (f FileStatus) Unstaged() bool {
	return f.Worktree != ' ' || f.Index == '?'
}

// Label returns Git's two-letter status, such as "M " or "??".
func (f FileStatus) Label() string {
	if f.Index == '?' {
		return "??"
	}
	return string([]byte{f.Index, f.Worktree})
}

// DisplayPath returns the path label shown to users.
func (f FileStatus) DisplayPath() string {
	if f.OldPath != "" {
		return f.OldPath + " -> " + f.Path
	}
	if f.Directory {
		return strings.TrimSuffix(f.Path, "/") + "/"
	}
	return f.Path
}

// GitPaths returns the path arguments Git needs for this status row.
func (f FileStatus) GitPaths() []string {
	if f.OldPath != "" {
		return []string{f.OldPath, f.Path}
	}
	if f.Path == "" {
		return nil
	}
	return []string{f.Path}
}

// Deleted reports whether this row represents a removed tracked path.
func (f FileStatus) Deleted() bool {
	return f.Index == 'D' || f.Worktree == 'D'
}

// Rename reports whether this row represents a rename or copy.
func (f FileStatus) Renamed() bool {
	return f.OldPath != ""
}

// RepoInfo is the repository summary shown in the top status bar.
type RepoInfo struct {
	Repo        string
	Branch      string
	Upstream    string
	User        string
	Ahead       int
	Behind      int
	Staged      int
	Unstaged    int
	LastPull    time.Time
	LastPullSet bool
}

// NewRunner creates a runner that executes Git in dir, or in the current directory.
func NewRunner(dir string) Runner {
	return Runner{Dir: dir}
}
