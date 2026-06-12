package git

import "time"

// Runner is the app's doorway to the installed git binary. The TUI calls
// Runner methods instead of building raw git commands itself.
type Runner struct {
	Dir string
}

// FileStatus is one changed-file row from `git status --porcelain`.
type FileStatus struct {
	Path     string
	Index    byte
	Worktree byte
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
