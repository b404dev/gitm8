package tui

import (
	"bytes"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// highlightPreview tokenizes the complete file before reattaching gitm8's
// numbered preview rows, so multiline constructs retain their lexer state.
func highlightPreview(path, content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != path {
		return content
	}
	type numberedLine struct {
		index        int
		prefix, code string
	}
	var numbered []numberedLine
	for i := 2; i < len(lines); i++ {
		idx := strings.Index(lines[i], "  ")
		if idx <= 0 || !isLineNumberPrefix(lines[i][:idx]) {
			continue
		}
		numbered = append(numbered, numberedLine{index: i, prefix: lines[i][:idx], code: lines[i][idx+2:]})
	}
	if len(numbered) == 0 {
		return content
	}
	codeLines := make([]string, len(numbered))
	for i, line := range numbered {
		codeLines[i] = line.code
	}
	highlighted, err := chromaHighlight(path, strings.Join(codeLines, "\n"))
	if err != nil {
		return content
	}
	highlightedLines := strings.Split(strings.TrimSuffix(highlighted, "\n"), "\n")
	if len(highlightedLines) != len(numbered) {
		return content
	}
	lines[0] = titleStyle.Render(lines[0])
	for i, line := range numbered {
		lines[line.index] = mutedStyle.Render(line.prefix) + "  " + highlightedLines[i]
	}
	return strings.Join(lines, "\n")
}

func chromaHighlight(path, source string) (string, error) {
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Analyse(source)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, source)
	if err != nil {
		return "", err
	}
	style := styles.Get(chromaThemeName(activeThemeName))
	if style == nil {
		style = styles.Fallback
	}
	var output bytes.Buffer
	if err := formatters.TTY16m.Format(&output, style, iterator); err != nil {
		return "", err
	}
	return output.String(), nil
}

func chromaThemeName(theme string) string {
	switch theme {
	case "catppuccin", "catppuccin-mocha":
		return "catppuccin-mocha"
	case "catppuccin-macchiato", "catppuccin-frappe", "catppuccin-latte":
		return theme
	case "midnight", "violet":
		return "tokyonight-night"
	case "ocean", "cyan", "steel":
		return "github-dark"
	case "forest", "lime":
		return "evergarden"
	case "amber":
		return "gruvbox"
	case "rose":
		return "rose-pine"
	case "mono":
		return "bw"
	case "highvis":
		return "hr_high_contrast"
	default:
		return "dracula"
	}
}

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
