package git

import (
	"context"
	"fmt"
	"strings"
)

// Create And Switch

// CreateBranch creates and switches to a new branch without returning output.
func (r Runner) CreateBranch(ctx context.Context, name string) error {
	return r.run(ctx, "switch", "-c", name)
}

// CreateBranchOutput creates a branch and returns Git's normal output.
func (r Runner) CreateBranchOutput(ctx context.Context, name string) (string, error) {
	return r.output(ctx, "switch", "-c", name)
}

// SwitchBranch switches to an existing branch without returning output.
func (r Runner) SwitchBranch(ctx context.Context, name string) error {
	return r.run(ctx, "switch", name)
}

// SwitchBranchOutput switches branches and returns Git's normal output.
func (r Runner) SwitchBranchOutput(ctx context.Context, name string) (string, error) {
	return r.output(ctx, "switch", name)
}

// SwitchBranchWithChangesOutput moves local changes to another branch by using
// git stash before the switch and git stash pop after it.
func (r Runner) SwitchBranchWithChangesOutput(ctx context.Context, name string) (string, error) {
	if strings.TrimSpace(r.bestEffort(ctx, "status", "--porcelain", "-uall")) == "" {
		return r.SwitchBranchOutput(ctx, name)
	}

	var outputs []string
	stash, err := r.output(ctx, "stash", "push", "-u", "-m", "gitm8: carry changes to "+name)
	if err != nil {
		return stash, err
	}
	outputs = append(outputs, stash)

	switched, err := r.SwitchBranchOutput(ctx, name)
	if err != nil {
		restore, restoreErr := r.output(ctx, "stash", "pop")
		if restoreErr != nil {
			return joinOutput(outputs...), fmt.Errorf("%w; also failed to restore stashed changes: %v", err, restoreErr)
		}
		outputs = append(outputs, restore)
		return joinOutput(outputs...), err
	}
	outputs = append(outputs, switched)

	popped, err := r.output(ctx, "stash", "pop")
	outputs = append(outputs, popped)
	if err != nil {
		return joinOutput(outputs...), err
	}
	outputs = append(outputs, "Switched to "+name+" with current changes")
	return joinOutput(outputs...), nil
}

// Delete

// DeleteBranch deletes a local branch using Git's normal safety checks.
func (r Runner) DeleteBranch(ctx context.Context, name string) error {
	return r.run(ctx, "branch", "-d", name)
}

// DeleteBranchOutput force deletes a local branch and removes stale remote
// branch references with the same name.
func (r Runner) DeleteBranchOutput(ctx context.Context, name string) (string, error) {
	var parts []string
	local, err := r.output(ctx, "branch", "-D", name)
	if err == nil {
		if local = strings.TrimSpace(local); local != "" {
			parts = append(parts, local)
		}
	}

	for _, remote := range nonEmptyLines(r.bestEffort(ctx, "remote")) {
		pruned := strings.TrimSpace(r.bestEffort(ctx, "branch", "-dr", remote+"/"+name))
		if pruned != "" {
			parts = append(parts, pruned)
		}
	}

	if len(parts) == 0 {
		return local, err
	}
	return strings.Join(parts, "\n"), nil
}

// DeleteRemoteBranchOutput deletes the branch from the default remote. If it
// was already gone, it cleans up the stale local reference.
func (r Runner) DeleteRemoteBranchOutput(ctx context.Context, name string) (string, error) {
	remote := r.defaultRemote(ctx)
	if remote == "" {
		return "", fmt.Errorf("no remote configured")
	}

	out, err := r.output(ctx, "push", remote, "--delete", name)
	if err != nil {
		if pruned, pruneErr := r.output(ctx, "branch", "-dr", remote+"/"+name); pruneErr == nil {
			return "Remote branch already gone; pruned " + remote + "/" + name + "\n" + strings.TrimSpace(pruned), nil
		}
		return out, err
	}
	return out, nil
}

// List And Normalize

// Branches returns local branches first, then remote-only branch names without
// the remote prefix, so the picker stays easy to read.
func (r Runner) Branches(ctx context.Context) ([]string, error) {
	out, err := r.output(ctx, "branch", "-a", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	remotes := nonEmptyLines(r.bestEffort(ctx, "remote"))

	seen := map[string]bool{}
	var locals, remoteOnly []string
	for _, line := range nonEmptyLines(out) {
		if strings.Contains(line, "->") || strings.HasSuffix(line, "/HEAD") || contains(remotes, line) {
			continue
		}
		name, isRemote := stripRemote(line, remotes)
		if isRemote {
			remoteOnly = append(remoteOnly, name)
			continue
		}
		seen[name] = true
		locals = append(locals, name)
	}

	branches := locals
	for _, name := range remoteOnly {
		if !seen[name] {
			seen[name] = true
			branches = append(branches, name)
		}
	}
	return branches, nil
}

// defaultRemote returns origin when it exists, otherwise the first configured remote.
func (r Runner) defaultRemote(ctx context.Context) string {
	remotes := nonEmptyLines(r.bestEffort(ctx, "remote"))
	if contains(remotes, "origin") {
		return "origin"
	}
	if len(remotes) > 0 {
		return remotes[0]
	}
	return ""
}

// stripRemote turns "origin/name" into "name" and reports whether it changed it.
func stripRemote(name string, remotes []string) (string, bool) {
	for _, remote := range remotes {
		if strings.HasPrefix(name, remote+"/") {
			return strings.TrimPrefix(name, remote+"/"), true
		}
	}
	return name, false
}
