package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// run executes git and ignores normal output, but still returns useful errors.
func (r Runner) run(ctx context.Context, args ...string) error {
	_, err := r.output(ctx, args...)
	return err
}

// output executes git and returns normal output for the UI to show.
func (r Runner) output(ctx context.Context, args ...string) (string, error) {
	return r.commandOutput(ctx, "git", args...)
}

// commandOutput starts a git or gh process, captures output, and turns failures
// into readable errors.
func (r Runner) commandOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}

	// gh commands are used from inside the TUI, where interactive prompts
	// would look broken. Fail fast instead and show the user the CLI message.
	if name == "gh" {
		cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(combinedOutput(stdout.String(), stderr.String()))
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), detail)
	}
	return stdout.String(), nil
}

// bestEffort reads optional Git details. If Git fails, it returns an empty string.
func (r Runner) bestEffort(ctx context.Context, args ...string) string {
	out, err := r.output(ctx, args...)
	if err != nil {
		return ""
	}
	return out
}
