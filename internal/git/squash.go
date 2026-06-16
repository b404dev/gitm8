package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/b404dev/gitm8/internal/logging"
)

// squashFieldSep separates the short hash from the subject in the log format.
// A unit separator never appears in commit text, so it parses cleanly.
const squashFieldSep = "\x1f"

// SquashAction is one commit's selection in the in-TUI squash plan. The actions
// are kept in the same newest-first order the picker shows. Base is the commit
// that receives all commits marked Squash.
type SquashAction struct {
	Hash   string
	Squash bool
	Base   bool
}

// SquashPlanLine is the resolved action ("pick" or "squash") for one commit.
type SquashPlanLine struct {
	Hash   string
	Action string
}

// ResolveSquashPlan turns per-commit selections (newest-first) into explicit
// pick/squash actions. The selected base stays "pick"; every other commit
// marked Squash becomes "squash". A commit cannot be both base and squash, so
// base wins. If no base was selected, every commit stays "pick".
func ResolveSquashPlan(actions []SquashAction) []SquashPlanLine {
	lines := make([]SquashPlanLine, len(actions))
	base := squashBaseIndex(actions)
	for i := range actions {
		action := "pick"
		if base >= 0 && i != base && actions[i].Squash {
			action = "squash"
		}
		lines[i] = SquashPlanLine{Hash: actions[i].Hash, Action: action}
	}
	return lines
}

func squashBaseIndex(actions []SquashAction) int {
	for i, action := range actions {
		if action.Base {
			return i
		}
	}
	return -1
}

// SquashCandidates returns the commits on the current branch that sit ahead of
// base, newest first. These are the commits the in-TUI squash picker offers to
// collapse. It validates the same preconditions a manual interactive rebase
// would, so the picker can show a clear reason instead of a broken screen.
func (r Runner) SquashCandidates(ctx context.Context, base string) ([]Commit, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "main"
	}

	branch := strings.TrimSpace(r.bestEffort(ctx, "branch", "--show-current"))
	if branch == "" {
		return nil, fmt.Errorf("cannot squash commits while detached")
	}
	if branch == base {
		return nil, fmt.Errorf("cannot squash commits on the default branch %q", base)
	}

	baseRef := r.squashBaseRef(ctx, base)
	if baseRef == "" {
		return nil, fmt.Errorf("could not find base branch %q or %q", base, "origin/"+base)
	}

	status := strings.TrimSpace(r.bestEffort(ctx, "status", "--porcelain", "-uall"))
	if status != "" {
		return nil, fmt.Errorf("working tree must be clean before squashing")
	}

	// List from the merge-base, not the base tip, so a base branch that has
	// moved on since this branch forked does not pull unrelated commits into the
	// picker.
	mergeBase, err := r.squashMergeBase(ctx, baseRef)
	if err != nil {
		return nil, err
	}

	out, err := r.output(ctx, "log", "--pretty=format:%h"+squashFieldSep+"%s", mergeBase+"..HEAD")
	if err != nil {
		return nil, err
	}

	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, squashFieldSep, 2)
		commit := Commit{Hash: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			commit.Subject = parts[1]
		}
		commits = append(commits, commit)
	}

	if len(commits) == 0 {
		return nil, fmt.Errorf("no commits to squash on %s compared with %s", branch, base)
	}

	logging.Info("git", "SquashCandidates", "squash_candidates_listed",
		logging.F("base", base),
		logging.F("base_ref", baseRef),
		logging.F("branch", branch),
		logging.F("commit_count", len(commits)),
	)
	return commits, nil
}

// ApplySquash rewrites the current branch by rebuilding its commits from the
// merge-base. It keeps unmarked commits as separate cherry-picks and combines
// the selected base plus every commit marked Squash into one commit using the
// base commit's message. actions are newest first, the same order shown in the
// picker.
func (r Runner) ApplySquash(ctx context.Context, base string, actions []SquashAction) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "main"
	}
	if len(actions) < 2 {
		return "", fmt.Errorf("need at least two commits to squash")
	}
	baseIndex := squashBaseIndex(actions)
	if baseIndex < 0 {
		return "", fmt.Errorf("choose a base commit before squashing")
	}
	squashes := squashCount(actions, baseIndex)
	if squashes == 0 {
		return "", fmt.Errorf("select at least one commit to squash into the base")
	}

	baseRef := r.squashBaseRef(ctx, base)
	if baseRef == "" {
		return "", fmt.Errorf("could not find base branch %q or %q", base, "origin/"+base)
	}

	// Rebase onto the merge-base, not the base tip. This squashes the branch's
	// own commits in place without also replaying them onto a base branch that
	// has advanced — which would cause conflicts and pull in unrelated history.
	mergeBase, err := r.squashMergeBase(ctx, baseRef)
	if err != nil {
		return "", err
	}

	status := strings.TrimSpace(r.bestEffort(ctx, "status", "--porcelain", "-uall"))
	if status != "" {
		return "", fmt.Errorf("working tree must be clean before squashing")
	}
	originalHead := strings.TrimSpace(r.bestEffort(ctx, "rev-parse", "HEAD"))
	if originalHead == "" {
		return "", fmt.Errorf("could not resolve current HEAD before squashing")
	}

	oldestFirst := make([]SquashAction, 0, len(actions))
	for i := len(actions) - 1; i >= 0; i-- {
		oldestFirst = append(oldestFirst, actions[i])
	}

	logging.Info("git", "ApplySquash", "squash_begin",
		logging.F("base", base),
		logging.F("base_ref", baseRef),
		logging.F("merge_base", mergeBase),
		logging.F("commits", len(actions)),
		logging.F("squashes", squashes),
	)

	var outputs []string
	resetOut, err := r.output(ctx, "reset", "--hard", mergeBase)
	outputs = append(outputs, resetOut)
	if err != nil {
		return joinOutput(outputs...), err
	}

	combinedWritten := false
	for _, action := range oldestFirst {
		switch {
		case action.Base:
			for _, selected := range oldestFirst {
				if selected.Base || selected.Squash {
					out, err := r.output(ctx, "cherry-pick", "--no-commit", selected.Hash)
					outputs = append(outputs, out)
					if err != nil {
						return r.restoreAfterSquashFailure(ctx, originalHead, outputs, err)
					}
				}
			}
			out, err := r.output(ctx, "commit", "-C", action.Hash)
			outputs = append(outputs, out)
			if err != nil {
				return r.restoreAfterSquashFailure(ctx, originalHead, outputs, err)
			}
			combinedWritten = true
		case action.Squash:
			continue
		default:
			out, err := r.output(ctx, "cherry-pick", action.Hash)
			outputs = append(outputs, out)
			if err != nil {
				return r.restoreAfterSquashFailure(ctx, originalHead, outputs, err)
			}
		}
	}
	if !combinedWritten {
		return r.restoreAfterSquashFailure(ctx, originalHead, outputs, fmt.Errorf("could not find selected base commit in squash plan"))
	}

	outputs = append(outputs, fmt.Sprintf("Squashed %d commit(s) into %s", squashes, actions[baseIndex].Hash))
	logging.Info("git", "ApplySquash", "squash_complete", logging.F("squashes", squashes))
	return joinOutput(outputs...), nil
}

func squashCount(actions []SquashAction, base int) int {
	count := 0
	for i, action := range actions {
		if i == base {
			continue
		}
		if action.Squash {
			count++
		}
	}
	return count
}

func (r Runner) restoreAfterSquashFailure(ctx context.Context, originalHead string, outputs []string, cause error) (string, error) {
	_ = r.run(ctx, "cherry-pick", "--abort")
	restore, restoreErr := r.output(ctx, "reset", "--hard", originalHead)
	outputs = append(outputs, restore)
	if restoreErr != nil {
		return joinOutput(outputs...), fmt.Errorf("%w; also failed to restore original HEAD: %v", cause, restoreErr)
	}
	return joinOutput(outputs...), cause
}

// squashMergeBase returns the commit where the current branch diverged from
// baseRef. Squashing rebuilds from this point so only the branch's own commits
// are rewritten.
func (r Runner) squashMergeBase(ctx context.Context, baseRef string) (string, error) {
	out, err := r.output(ctx, "merge-base", "HEAD", baseRef)
	if err != nil {
		return "", err
	}
	mergeBase := strings.TrimSpace(out)
	if mergeBase == "" {
		return "", fmt.Errorf("could not find the point where this branch left %s", baseRef)
	}
	return mergeBase, nil
}

// squashBaseRef resolves base to a usable commit-ish, falling back to the remote
// tracking branch when the local branch is missing.
func (r Runner) squashBaseRef(ctx context.Context, base string) string {
	if strings.TrimSpace(r.bestEffort(ctx, "rev-parse", "--verify", "--quiet", base+"^{commit}")) != "" {
		return base
	}
	remoteBase := "origin/" + base
	if strings.TrimSpace(r.bestEffort(ctx, "rev-parse", "--verify", "--quiet", remoteBase+"^{commit}")) != "" {
		return remoteBase
	}
	return ""
}
