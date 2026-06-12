package git

import (
	"context"
	"strings"
)

// SyncOutput performs the dashboard sync action: pull --rebase, then push.
func (r Runner) SyncOutput(ctx context.Context) (string, error) {
	pulled, err := r.output(ctx, "pull", "--rebase")
	if err != nil {
		return joinOutput(
			pulled,
			"Sync stopped during pull --rebase.",
			"Resolve conflicts, then run: git rebase --continue",
			"To cancel, run: git rebase --abort",
		), err
	}

	pushed, err := r.pushOutputWithRepoURL(ctx)
	if err != nil {
		return joinOutput(pulled, pushed), err
	}

	out := joinOutput(pulled, pushed, "Sync complete")
	if out == "" {
		out = "Sync complete"
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}
