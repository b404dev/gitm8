package tui

import "strings"

// selectedPathFromTarget returns an empty path when the whole repo is selected.
func selectedPathFromTarget(target string) string {
	if target == "repo" {
		return ""
	}
	return target
}

// selectedPath returns the selected path in the form Git commands expect.
func (m Model) selectedPath() string {
	return selectedPathFromTarget(m.target)
}

// optionalPath returns no args for the whole repo, or one arg for one path.
func optionalPath(path string) []string {
	if path == "" {
		return nil
	}
	return []string{path}
}

// firstErr returns the first error from a set of load steps.
func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// trimMiddle shortens long labels while keeping their beginning and ending visible.
func trimMiddle(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}
	left := (width - 3) / 2
	right := width - 3 - left
	return value[:left] + "..." + value[len(value)-right:]
}

// oneLine collapses command output into a compact status-bar summary.
func oneLine(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

// visibleOutputLines wraps command output and keeps the newest lines.
func visibleOutputLines(value string, width int, maxLines int) ([]string, int) {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(value), "\n") {
		wrapped := wrapLine(strings.TrimRight(line, "\r"), width)
		lines = append(lines, wrapped...)
	}
	if len(lines) == 0 {
		return []string{""}, 0
	}
	if len(lines) <= maxLines {
		return lines, 0
	}
	return lines[len(lines)-maxLines:], len(lines) - maxLines
}

// wrapLine breaks one output line at whitespace when possible.
func wrapLine(line string, width int) []string {
	if width <= 0 {
		return []string{line}
	}
	if line == "" {
		return []string{""}
	}

	var lines []string
	for len(line) > width {
		breakAt := width
		if idx := strings.LastIndexAny(line[:width], " \t"); idx > 0 {
			breakAt = idx
		}
		lines = append(lines, strings.TrimRight(line[:breakAt], " \t"))
		line = strings.TrimLeft(line[breakAt:], " \t")
	}
	lines = append(lines, line)
	return lines
}

// joinGitOutput combines multi-step Git outputs for one dashboard action.
func joinGitOutput(outputs ...string) string {
	var parts []string
	for _, output := range outputs {
		output = strings.TrimSpace(output)
		if output != "" {
			parts = append(parts, output)
		}
	}
	return strings.Join(parts, "\n")
}

// clamp bounds cursor and layout values to valid ranges.
func clamp(value, minValue, maxValue int) int {
	return min(max(value, minValue), maxValue)
}
