package git

import (
	"context"
)

// Rebase starts a rebase onto the selected target branch.
func (r Runner) Rebase(ctx context.Context, target string) error {
	return r.run(ctx, "rebase", target)
}

// RebaseOutput starts a rebase and returns Git's normal output for the UI.
func (r Runner) RebaseOutput(ctx context.Context, target string) (string, error) {
	return r.output(ctx, "rebase", target)
}

// RebaseContinueOutput continues an in-progress rebase after conflicts are resolved.
func (r Runner) RebaseContinueOutput(ctx context.Context) (string, error) {
	return r.output(ctx, "rebase", "--continue")
}

// RebaseAbortOutput aborts an in-progress rebase and returns Git output.
func (r Runner) RebaseAbortOutput(ctx context.Context) (string, error) {
	return r.output(ctx, "rebase", "--abort")
}

// RebaseSkipOutput skips the current patch in an in-progress rebase.
func (r Runner) RebaseSkipOutput(ctx context.Context) (string, error) {
	return r.output(ctx, "rebase", "--skip")
}
