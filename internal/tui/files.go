package tui

import (
	pathpkg "path"
	"strings"

	"github.com/b404dev/gitm8/internal/git"
)

// filteredFiles returns the changed-file rows that match the current file filter.
func (m Model) filteredFiles() []git.FileStatus {
	query := strings.TrimSpace(m.fileFilter.Value())
	if query == "" {
		return m.files
	}

	matches := make([]git.FileStatus, 0, len(m.files))
	for _, file := range m.files {
		if fileFilterMatches(file, query) {
			matches = append(matches, file)
		}
	}
	return matches
}

func fileFilterMatches(file git.FileStatus, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}

	displayPath := strings.ToLower(file.DisplayPath())
	label := strings.ToLower(file.Label())
	if strings.ContainsAny(query, "*?") {
		return wildcardMatch(displayPath, query) || wildcardMatch(label, query)
	}
	return strings.Contains(displayPath, query) || strings.Contains(label, query)
}

func fileListName(file git.FileStatus) string {
	if file.OldPath != "" {
		return pathpkg.Base(file.OldPath) + " -> " + pathpkg.Base(file.Path)
	}
	name := pathpkg.Base(strings.TrimSuffix(file.Path, "/"))
	if file.Directory {
		return name + "/"
	}
	return name
}
