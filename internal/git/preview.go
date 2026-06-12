package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Preview returns readable file, directory, binary, or deleted-file content.
func (r Runner) Preview(ctx context.Context, path string) (string, error) {
	if path == "" {
		return r.Review(ctx)
	}

	workingPath := r.workingPath(path)
	if stat, err := os.Stat(workingPath); err == nil && stat.IsDir() {
		return previewDir(path, workingPath)
	}

	content, err := os.ReadFile(workingPath)
	if err != nil {
		committed, err := r.output(ctx, "show", "HEAD:"+path)
		if err != nil {
			return fmt.Sprintf("Unable to preview %s\n\nThe file may be deleted, renamed, or outside the working tree.\n", path), nil
		}
		return numbered(path, committed), nil
	}

	if bytes.Contains(content, []byte{0}) {
		return fmt.Sprintf("Binary file: %s\n", path), nil
	}
	return numbered(path, string(content)), nil
}

// previewDir renders a bounded, sorted directory listing for the preview pane.
func previewDir(path string, workingPath string) (string, error) {
	entries, err := os.ReadDir(workingPath)
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	var b strings.Builder
	b.WriteString(path)
	b.WriteString("\n\n")
	if len(entries) == 0 {
		b.WriteString("Empty directory\n")
		return b.String(), nil
	}

	for i, entry := range entries {
		if i >= 200 {
			fmt.Fprintf(&b, "\n... %d more entries\n", len(entries)-i)
			break
		}
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		fmt.Fprintf(&b, "%3d  %s\n", i+1, name)
	}
	return b.String(), nil
}
