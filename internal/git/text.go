package git

import (
	"fmt"
	"strings"
)

// combinedOutput joins normal output and error output into one message.
func combinedOutput(stdout string, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	switch {
	case stdout == "":
		return stderr
	case stderr == "":
		return stdout
	default:
		return stdout + "\n" + stderr
	}
}

// joinOutput joins non-empty command output parts with newlines.
func joinOutput(outputs ...string) string {
	var parts []string
	for _, output := range outputs {
		output = strings.TrimSpace(output)
		if output != "" {
			parts = append(parts, output)
		}
	}
	return strings.Join(parts, "\n")
}

// nonEmptyLines returns trimmed lines and drops blank separators.
func nonEmptyLines(value string) []string {
	lines := strings.Split(value, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// contains checks small string slices used for branch and remote lists.
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// numbered formats file content with a header and stable line numbers.
func numbered(path string, content string) string {
	lines := strings.Split(content, "\n")
	width := len(fmt.Sprintf("%d", len(lines)))

	var b strings.Builder
	b.WriteString(path)
	b.WriteString("\n\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			continue
		}
		fmt.Fprintf(&b, "%*d  %s\n", width, i+1, line)
	}
	return b.String()
}
