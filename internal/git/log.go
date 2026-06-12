package git

import (
	"context"
	"strconv"
	"strings"
)

// Log returns a compact commit history for the dashboard log viewer.
func (r Runner) Log(ctx context.Context, limit int, graph bool) (string, error) {
	if limit <= 0 {
		limit = 30
	}

	args := []string{
		"log",
		"--decorate",
		"--date=short",
		"--pretty=format:%h  %ad  %d %s  (%an)",
		"-n",
		strconv.Itoa(limit),
	}
	if graph {
		args = append([]string{"log", "--graph"}, args[1:]...)
	}

	out, err := r.output(ctx, args...)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "No commits found.\n", nil
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}
