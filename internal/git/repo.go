package git

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RepoInfo gathers the repository details shown in the top status bar.
func (r Runner) RepoInfo(ctx context.Context) (RepoInfo, error) {
	files, filesErr := r.FileStatuses(ctx)
	info := RepoInfo{}
	for _, file := range files {
		if file.Staged() {
			info.Staged++
		}
		if file.Unstaged() {
			info.Unstaged++
		}
	}

	if root := strings.TrimSpace(r.bestEffort(ctx, "rev-parse", "--show-toplevel")); root != "" {
		info.Repo = filepath.Base(root)
	}
	info.Branch = strings.TrimSpace(r.bestEffort(ctx, "branch", "--show-current"))
	if info.Branch == "" {
		info.Branch = "detached"
	}
	info.Upstream = strings.TrimSpace(r.bestEffort(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"))
	info.User = strings.TrimSpace(r.bestEffort(ctx, "config", "user.name"))
	if info.User == "" {
		info.User = strings.TrimSpace(r.bestEffort(ctx, "config", "user.email"))
	}
	if info.User == "" {
		info.User = "unknown user"
	}

	if info.Upstream != "" {
		info.Ahead, info.Behind = r.aheadBehind(ctx)
	}
	if t, ok := r.lastFetchHeadTime(ctx); ok {
		info.LastPull = t
		info.LastPullSet = true
	}
	return info, filesErr
}

// FileStatuses reads `git status --porcelain` and turns it into file rows.
func (r Runner) FileStatuses(ctx context.Context) ([]FileStatus, error) {
	out, err := r.output(ctx, "status", "--porcelain=v1", "-z", "-uall")
	if err != nil {
		return nil, err
	}

	return parseStatusZ(out), nil
}

func parseStatusZ(out string) []FileStatus {
	var files []FileStatus
	records := strings.Split(out, "\x00")
	for i := 0; i < len(records); i++ {
		record := records[i]
		if record == "" {
			continue
		}
		status, ok := parseStatusRecord(record)
		if !ok {
			continue
		}
		if status.Index == 'R' || status.Index == 'C' {
			if i+1 < len(records) {
				status.OldPath = records[i+1]
				i++
			}
		}
		files = append(files, status)
	}
	return files
}

// parseStatusRecord reads one NUL-delimited `git status --porcelain=v1 -z`
// record in "XY PATH" form. Rename records carry the old path in the next
// NUL-delimited record and are completed by parseStatusZ.
func parseStatusRecord(record string) (FileStatus, bool) {
	if len(record) < 4 {
		return FileStatus{}, false
	}

	return FileStatus{
		Path:     record[3:],
		Index:    record[0],
		Worktree: record[1],
	}, true
}

// Files returns only the file paths from Git status.
func (r Runner) Files(ctx context.Context) ([]string, error) {
	statuses, err := r.FileStatuses(ctx)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(statuses))
	for _, status := range statuses {
		files = append(files, status.Path)
	}
	return files, nil
}

// aheadBehind returns how many commits the branch is ahead of or behind its remote branch.
func (r Runner) aheadBehind(ctx context.Context) (int, int) {
	out := strings.TrimSpace(r.bestEffort(ctx, "rev-list", "--left-right", "--count", "HEAD...@{u}"))
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0
	}

	ahead, _ := strconv.Atoi(parts[0])
	behind, _ := strconv.Atoi(parts[1])
	return ahead, behind
}

// lastFetchHeadTime reads when FETCH_HEAD was last updated, which usually means
// when the repo last fetched.
func (r Runner) lastFetchHeadTime(ctx context.Context) (time.Time, bool) {
	gitDir := strings.TrimSpace(r.bestEffort(ctx, "rev-parse", "--git-dir"))
	if gitDir == "" {
		return time.Time{}, false
	}

	path := gitDir
	if !filepath.IsAbs(path) {
		path = r.workingPath(path)
	}

	stat, err := os.Stat(filepath.Join(path, "FETCH_HEAD"))
	if err != nil {
		return time.Time{}, false
	}
	return stat.ModTime(), true
}

// workingPath turns a repo-relative path into a filesystem path when Runner.Dir is set.
func (r Runner) workingPath(path string) string {
	if r.Dir == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(r.Dir, path)
}
