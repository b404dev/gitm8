package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceRepositoriesFindsNestedRepos(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"alpha/.git", "group/beta/.git", "node_modules/ignored/.git"} {
		if err := os.MkdirAll(filepath.Join(root, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	repos, err := WorkspaceRepositories(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("repositories = %v, want alpha and group/beta", repos)
	}
}

func TestInitWorkspaceRepository(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Github")
	path, _, err := InitWorkspaceRepository(context.Background(), root, "demo", "main")
	if err != nil {
		t.Fatal(err)
	}
	if !NewRunner(path).IsRepository(context.Background()) {
		t.Fatal("initialized project is not a repository")
	}
	if _, _, err := InitWorkspaceRepository(context.Background(), root, "../outside", "main"); err == nil {
		t.Fatal("expected unsafe project name to be rejected")
	}
}

func TestWorkspaceRepositoriesDoesNotCreateMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	if _, err := WorkspaceRepositories(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("workspace scan created missing directory")
	}
}
