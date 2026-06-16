package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/b404dev/gitm8/internal/logging"
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

// commandOutput starts a git, gh, or helper process, captures output, and turns
// failures into readable errors.
func (r Runner) commandOutput(ctx context.Context, name string, args ...string) (string, error) {
	return r.commandOutputEnv(ctx, nil, name, args...)
}

// commandOutputEnv is commandOutput with extra environment variables appended.
// The squash workflow uses it to point git's sequence and commit editors at
// non-interactive helpers so rebasing never opens an external editor.
func (r Runner) commandOutputEnv(ctx context.Context, extraEnv []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	env := os.Environ()
	if len(extraEnv) > 0 {
		env = append(env, extraEnv...)
	}
	start := time.Now()
	logging.Debug("git", "commandOutput", "command_start",
		logging.F("name", name),
		logging.F("args", safeCommandArgs(args)),
		logging.F("dir", r.Dir),
	)

	// gh commands are used from inside the TUI, where interactive prompts
	// would look broken. Fail fast instead and show the user the CLI message.
	if name == "gh" {
		env = append(env, "GH_PROMPT_DISABLED=1")
	}
	if len(env) > 0 {
		cmd.Env = env
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
		logging.Error("git", "commandOutput", "command_failed",
			logging.F("name", name),
			logging.F("args", safeCommandArgs(args)),
			logging.F("duration_ms", time.Since(start).Milliseconds()),
			logging.F("stdout_bytes", stdout.Len()),
			logging.F("stderr_bytes", stderr.Len()),
			logging.F("error", detail),
		)
		return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), detail)
	}
	logging.Debug("git", "commandOutput", "command_complete",
		logging.F("name", name),
		logging.F("args", safeCommandArgs(args)),
		logging.F("duration_ms", time.Since(start).Milliseconds()),
		logging.F("stdout_bytes", stdout.Len()),
		logging.F("stderr_bytes", stderr.Len()),
	)
	return stdout.String(), nil
}

// commandInputOutput is commandOutput with stdin attached.
func (r Runner) commandInputOutput(ctx context.Context, input string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	cmd.Stdin = strings.NewReader(input)
	start := time.Now()
	logging.Info("git", "commandInputOutput", "command_start",
		logging.F("name", name),
		logging.F("args", safeCommandArgs(args)),
		logging.F("dir", r.Dir),
		logging.F("stdin_bytes", len(input)),
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(combinedOutput(stdout.String(), stderr.String()))
		if detail == "" {
			detail = err.Error()
		}
		logging.Error("git", "commandInputOutput", "command_failed",
			logging.F("name", name),
			logging.F("args", safeCommandArgs(args)),
			logging.F("duration_ms", time.Since(start).Milliseconds()),
			logging.F("stdin_bytes", len(input)),
			logging.F("stdout_bytes", stdout.Len()),
			logging.F("stderr_bytes", stderr.Len()),
			logging.F("error", detail),
		)
		return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), detail)
	}
	logging.Info("git", "commandInputOutput", "command_complete",
		logging.F("name", name),
		logging.F("args", safeCommandArgs(args)),
		logging.F("duration_ms", time.Since(start).Milliseconds()),
		logging.F("stdin_bytes", len(input)),
		logging.F("stdout_bytes", stdout.Len()),
		logging.F("stderr_bytes", stderr.Len()),
	)
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

func safeCommandArgs(args []string) string {
	safe := make([]string, 0, len(args))
	redactNext := false
	for _, arg := range args {
		if redactNext {
			safe = append(safe, "[text]")
			redactNext = false
			continue
		}
		safe = append(safe, arg)
		switch arg {
		case "-m", "--message", "--title", "--body":
			redactNext = true
		}
	}
	return strings.Join(safe, " ")
}
