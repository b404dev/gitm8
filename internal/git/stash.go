package git

import (
	"context"
	"fmt"
	"strings"
)

// Stashes returns the stash stack newest first.
func (r Runner) Stashes(ctx context.Context) ([]Stash, error) {
	out, err := r.output(ctx, "stash", "list", "--format=%gd%x09%gs")
	if err != nil {
		return nil, err
	}

	var stashes []Stash
	for _, line := range nonEmptyLines(out) {
		stash, ok := parseStashLine(line)
		if ok {
			stashes = append(stashes, stash)
		}
	}
	return stashes, nil
}

// StashDiff shows the full patch stored in one stash.
func (r Runner) StashDiff(ctx context.Context, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "No stash selected.\n", nil
	}
	out, err := r.output(ctx, "stash", "show", "--include-untracked", "--stat", "--patch", ref)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "Selected stash has no diff.\n", nil
	}
	return out, nil
}

// StashPushOutput saves current tracked and untracked changes.
func (r Runner) StashPushOutput(ctx context.Context) (string, error) {
	return r.output(ctx, "stash", "push", "-u", "-m", "gitm8 stash")
}

// StashApplyOutput applies a stash and keeps it in the stash stack.
func (r Runner) StashApplyOutput(ctx context.Context, ref string) (string, error) {
	return r.output(ctx, "stash", "apply", ref)
}

// StashPopOutput applies a stash and drops it when the apply succeeds.
func (r Runner) StashPopOutput(ctx context.Context, ref string) (string, error) {
	return r.output(ctx, "stash", "pop", ref)
}

// StashDropOutput removes one stash entry.
func (r Runner) StashDropOutput(ctx context.Context, ref string) (string, error) {
	return r.output(ctx, "stash", "drop", ref)
}

func parseStashLine(line string) (Stash, bool) {
	ref, subject, ok := strings.Cut(line, "\t")
	ref = strings.TrimSpace(ref)
	subject = strings.TrimSpace(subject)
	if !ok || ref == "" {
		return Stash{}, false
	}
	if subject == "" {
		subject = fmt.Sprintf("stash %s", ref)
	}
	return Stash{Ref: ref, Subject: subject}, true
}
