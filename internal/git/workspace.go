package git

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func (r Runner) IsRepository(ctx context.Context) bool {
	out, err := r.output(ctx, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

func WorkspaceRepositories(root string) ([]string, error) {
	root = filepath.Clean(root)
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var repos []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = len(strings.Split(rel, string(filepath.Separator)))
		}
		if entry.Name() == ".git" {
			repos = append(repos, filepath.Dir(path))
			return filepath.SkipDir
		}
		if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" || entry.Name() == "vendor" || depth > 3) {
			return filepath.SkipDir
		}
		return nil
	})
	sort.Slice(repos, func(i, j int) bool { return strings.ToLower(repos[i]) < strings.ToLower(repos[j]) })
	return repos, err
}

func InitWorkspaceRepository(ctx context.Context, root, name, defaultBranch string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return "", "", fmt.Errorf("project name must be one directory name")
	}
	path := filepath.Join(root, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", "", err
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		return "", "", err
	}
	out, err := NewRunner(path).output(ctx, "init", "-b", defaultBranch)
	return path, out, err
}

func CloneWorkspaceRepository(ctx context.Context, root, repository string) (string, string, error) {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return "", "", fmt.Errorf("enter a GitHub repository such as owner/project")
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return "", "", fmt.Errorf("GitHub CLI is not installed or not on PATH")
	}
	name := strings.TrimSuffix(filepath.Base(repository), ".git")
	if name == "" || name == "." || name == ".." {
		return "", "", fmt.Errorf("invalid GitHub repository")
	}
	destination := filepath.Join(root, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", "", err
	}
	out, err := NewRunner(root).commandOutput(ctx, "gh", "repo", "clone", repository, destination)
	return destination, out, err
}
