package tui

import (
	"fmt"
	"strconv"
	"strings"
)

type searchMatch struct {
	LineIndex int
	LineNo    int
	Text      string
	Score     int
}

func fuzzySearchLines(content string, query string, limit int) []searchMatch {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || limit <= 0 {
		return nil
	}

	lines := strings.Split(content, "\n")
	matches := make([]searchMatch, 0, min(limit, len(lines)))
	for i, line := range lines {
		score, ok := fuzzyLineScore(line, query)
		if !ok {
			continue
		}
		lineNo := previewLineNumber(line, i)
		matches = append(matches, searchMatch{
			LineIndex: i,
			LineNo:    lineNo,
			Text:      strings.TrimSpace(line),
			Score:     score,
		})
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func fuzzyLineScore(line string, query string) (int, bool) {
	ranges := searchRanges(line, query)
	if len(ranges) == 0 {
		return 0, false
	}
	return 10000 - ranges[0].start*20 - len(line), true
}

func previewLineNumber(line string, fallback int) int {
	trimmed := strings.TrimLeft(line, " ")
	parts := strings.SplitN(trimmed, "  ", 2)
	if len(parts) == 2 {
		if n, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
			return n
		}
	}
	return fallback + 1
}

func (m Model) searchView() string {
	query := strings.TrimSpace(m.searchInput.Value())
	content := highlightPreview(m.target, m.viewerContent)
	if query == "" {
		return content
	}

	lines := strings.Split(m.viewerContent, "\n")
	activeLine := -1
	if len(m.searchMatches) > 0 {
		activeLine = m.searchMatches[m.searchCursor].LineIndex
	}
	for i, line := range lines {
		lines[i] = highlightSearchLine(line, query, i == activeLine)
	}
	return strings.Join(lines, "\n")
}

func (m Model) searchSummary(query string) string {
	if len(m.searchMatches) == 0 {
		return fmt.Sprintf("no matches for %q", query)
	}
	return fmt.Sprintf("%d/%d matches  line %d", m.searchCursor+1, len(m.searchMatches), m.searchMatches[m.searchCursor].LineNo)
}

func highlightSearchLine(line string, query string, active bool) string {
	ranges := searchRanges(line, query)
	if len(ranges) == 0 {
		return line
	}

	var b strings.Builder
	last := 0
	style := activeStyle
	if active {
		style = keyStyle
	}
	for _, r := range ranges {
		b.WriteString(line[last:r.start])
		b.WriteString(style.Render(line[r.start:r.end]))
		last = r.end
	}
	b.WriteString(line[last:])
	return b.String()
}

type searchRange struct {
	start int
	end   int
}

func searchRanges(line string, query string) []searchRange {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	if strings.ContainsAny(query, "*?") {
		return wildcardSearchRanges(line, query)
	}
	return exactWordSearchRanges(line, query)
}

func exactWordSearchRanges(line string, query string) []searchRange {
	lower := strings.ToLower(line)
	var ranges []searchRange
	for start := 0; start < len(lower); {
		idx := strings.Index(lower[start:], query)
		if idx < 0 {
			break
		}
		idx += start
		end := idx + len(query)
		if isSearchBoundaryBefore(lower, idx) && isSearchBoundaryAfter(lower, end) {
			ranges = append(ranges, searchRange{start: idx, end: end})
		}
		start = idx + max(1, len(query))
	}
	return ranges
}

func wildcardSearchRanges(line string, pattern string) []searchRange {
	lower := strings.ToLower(line)
	var ranges []searchRange
	for start := 0; start < len(lower); {
		for start < len(lower) && !isSearchWordChar(lower[start]) {
			start++
		}
		if start >= len(lower) {
			break
		}
		end := start + 1
		for end < len(lower) && isSearchWordChar(lower[end]) {
			end++
		}
		if wildcardMatch(lower[start:end], pattern) {
			ranges = append(ranges, searchRange{start: start, end: end})
		}
		start = end
	}
	return ranges
}

func wildcardMatch(value string, pattern string) bool {
	valueIndex, patternIndex := 0, 0
	starIndex, matchIndex := -1, 0
	for valueIndex < len(value) {
		switch {
		case patternIndex < len(pattern) && (pattern[patternIndex] == '?' || pattern[patternIndex] == value[valueIndex]):
			valueIndex++
			patternIndex++
		case patternIndex < len(pattern) && pattern[patternIndex] == '*':
			starIndex = patternIndex
			matchIndex = valueIndex
			patternIndex++
		case starIndex != -1:
			patternIndex = starIndex + 1
			matchIndex++
			valueIndex = matchIndex
		default:
			return false
		}
	}
	for patternIndex < len(pattern) && pattern[patternIndex] == '*' {
		patternIndex++
	}
	return patternIndex == len(pattern)
}

func isSearchBoundaryBefore(value string, index int) bool {
	if index <= 0 {
		return true
	}
	return !isSearchWordChar(value[index-1])
}

func isSearchBoundaryAfter(value string, index int) bool {
	if index >= len(value) {
		return true
	}
	return !isSearchWordChar(value[index])
}

func isSearchWordChar(ch byte) bool {
	return ch == '_' || ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z'
}

func (m *Model) refreshSearchResults() {
	m.searchMatches = fuzzySearchLines(m.viewerContent, m.searchInput.Value(), 200)
	m.searchCursor = clamp(m.searchCursor, 0, max(0, len(m.searchMatches)-1))
	m.ensureSearchCursorVisible()
}

func (m *Model) ensureSearchCursorVisible() {
	visibleRows := max(1, max(8, m.height-8)-4)
	if m.searchCursor < m.searchOffset {
		m.searchOffset = m.searchCursor
	}
	if m.searchCursor >= m.searchOffset+visibleRows {
		m.searchOffset = m.searchCursor - visibleRows + 1
	}
	m.searchOffset = clamp(m.searchOffset, 0, max(0, len(m.searchMatches)-visibleRows))
}
