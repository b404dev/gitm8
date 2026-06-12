package tui

import (
	"path/filepath"
	"strings"
	"unicode"
)

// highlightPreview styles numbered preview output for supported text file types.
func highlightPreview(path string, content string) string {
	if !highlightablePath(path) {
		return content
	}

	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != path {
		return content
	}

	for i, line := range lines {
		if i == 0 {
			lines[i] = titleStyle.Render(line)
			continue
		}
		if i == 1 || line == "" {
			continue
		}
		idx := strings.Index(line, "  ")
		if idx <= 0 || !isLineNumberPrefix(line[:idx]) {
			continue
		}
		lines[i] = mutedStyle.Render(line[:idx]) + "  " + highlightCodeLine(path, line[idx+2:])
	}
	return strings.Join(lines, "\n")
}

// highlightablePath limits the lightweight highlighter to familiar text formats.
func highlightablePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".json", ".rs", ".py", ".rb", ".sh", ".bash", ".zsh",
		".java", ".c", ".h", ".cc", ".cpp", ".cs", ".swift", ".kt", ".php", ".css", ".scss", ".html",
		".xml", ".yaml", ".yml", ".toml", ".md", ".sql":
		return true
	default:
		return false
	}
}

// isLineNumberPrefix checks for the line numbers added to file previews.
func isLineNumberPrefix(value string) bool {
	hasDigit := false
	for _, r := range value {
		if unicode.IsDigit(r) {
			hasDigit = true
			continue
		}
		if r != ' ' {
			return false
		}
	}
	return hasDigit
}

// highlightCodeLine is intentionally small. It colors common strings, numbers,
// comments, and keywords without trying to fully parse each language.
func highlightCodeLine(path string, line string) string {
	commentStart := commentStartIndex(path, line)
	code := line
	comment := ""
	if commentStart >= 0 {
		code = line[:commentStart]
		comment = line[commentStart:]
	}

	var b strings.Builder
	for i := 0; i < len(code); {
		ch := code[i]
		if ch == '"' || ch == '\'' || ch == '`' {
			end := quotedEnd(code, i, ch)
			b.WriteString(syntaxStringStyle.Render(code[i:end]))
			i = end
			continue
		}
		if isIdentStart(ch) {
			end := i + 1
			for end < len(code) && isIdentPart(code[end]) {
				end++
			}
			word := code[i:end]
			if syntaxKeywords[word] {
				b.WriteString(syntaxKeywordStyle.Render(word))
			} else {
				b.WriteString(word)
			}
			i = end
			continue
		}
		if isDigit(ch) {
			end := i + 1
			for end < len(code) && (isDigit(code[end]) || code[end] == '.' || code[end] == '_') {
				end++
			}
			b.WriteString(syntaxNumberStyle.Render(code[i:end]))
			i = end
			continue
		}
		b.WriteByte(ch)
		i++
	}
	if comment != "" {
		b.WriteString(syntaxCommentStyle.Render(comment))
	}
	return b.String()
}

// commentStartIndex finds where a comment starts, ignoring comment markers inside strings.
func commentStartIndex(path string, line string) int {
	prefixes := commentPrefixes(path)
	if len(prefixes) == 0 {
		return -1
	}
	inString := byte(0)
	escaped := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && inString != '`' {
				escaped = true
				continue
			}
			if ch == inString {
				inString = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			inString = ch
			continue
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(line[i:], prefix) {
				return i
			}
		}
	}
	return -1
}

// commentPrefixes chooses the comment markers for a file type.
func commentPrefixes(path string) []string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py", ".rb", ".sh", ".bash", ".zsh", ".yaml", ".yml", ".toml":
		return []string{"#"}
	case ".html", ".xml", ".md":
		return []string{"<!--"}
	case ".css", ".scss":
		return []string{"/*"}
	case ".sql":
		return []string{"--"}
	default:
		return []string{"//"}
	}
}

// quotedEnd finds the end of a quoted string, including escaped characters.
func quotedEnd(value string, start int, quote byte) int {
	escaped := false
	for i := start + 1; i < len(value); i++ {
		ch := value[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && quote != '`' {
			escaped = true
			continue
		}
		if ch == quote {
			return i + 1
		}
	}
	return len(value)
}

// isIdentStart reports whether a character can start a word.
func isIdentStart(ch byte) bool {
	return ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}

// isIdentPart reports whether a character can continue a word.
func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}

// isDigit checks ASCII digits for number highlighting.
func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

var syntaxKeywords = map[string]bool{
	"as": true, "async": true, "await": true, "break": true, "case": true, "catch": true, "class": true,
	"const": true, "continue": true, "defer": true, "default": true, "def": true, "do": true, "else": true,
	"enum": true, "export": true, "extends": true, "fallthrough": true, "false": true, "finally": true,
	"for": true, "from": true, "func": true, "function": true, "go": true, "if": true, "import": true,
	"in": true, "interface": true, "let": true, "map": true, "match": true, "module": true, "mut": true,
	"nil": true, "none": true, "null": true, "package": true, "private": true, "protected": true, "public": true,
	"range": true, "return": true, "select": true, "self": true, "static": true, "struct": true, "switch": true,
	"this": true, "throw": true, "true": true, "try": true, "type": true, "var": true, "while": true, "yield": true,
}
