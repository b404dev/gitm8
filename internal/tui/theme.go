package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	keyStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	activeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("240"))

	syntaxKeywordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	syntaxStringStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	syntaxCommentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	syntaxNumberStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
)

// palette is one theme's color set before it is applied to Lip Gloss styles.
type palette struct {
	title     lipgloss.Color
	error     lipgloss.Color
	muted     lipgloss.Color
	key       lipgloss.Color
	border    lipgloss.Color
	active    lipgloss.Color
	keyword   lipgloss.Color
	stringLit lipgloss.Color
	comment   lipgloss.Color
	number    lipgloss.Color
}

var palettes = map[string]palette{
	"default":              {title: "212", error: "196", muted: "241", key: "86", border: "240", active: "229"},
	"ocean":                {title: "39", error: "203", muted: "244", key: "81", border: "31", active: "195"},
	"forest":               {title: "114", error: "203", muted: "244", key: "150", border: "65", active: "230"},
	"amber":                {title: "214", error: "203", muted: "245", key: "222", border: "94", active: "229"},
	"mono":                 {title: "255", error: "255", muted: "245", key: "252", border: "240", active: "255"},
	"rose":                 {title: "204", error: "196", muted: "245", key: "218", border: "132", active: "225"},
	"violet":               {title: "141", error: "203", muted: "245", key: "177", border: "61", active: "189"},
	"cyan":                 {title: "51", error: "203", muted: "244", key: "87", border: "37", active: "159"},
	"lime":                 {title: "154", error: "203", muted: "245", key: "190", border: "70", active: "229"},
	"steel":                {title: "75", error: "203", muted: "246", key: "117", border: "67", active: "252"},
	"highvis":              {title: "226", error: "196", muted: "250", key: "46", border: "226", active: "231"},
	"midnight":             {title: "111", error: "210", muted: "244", key: "153", border: "60", active: "195"},
	"catppuccin":           {title: "#cba6f7", error: "#f38ba8", muted: "#6c7086", key: "#89b4fa", border: "#45475a", active: "#f9e2af"},
	"catppuccin-mocha":     {title: "#cba6f7", error: "#f38ba8", muted: "#6c7086", key: "#89b4fa", border: "#45475a", active: "#f9e2af"},
	"catppuccin-macchiato": {title: "#c6a0f6", error: "#ed8796", muted: "#6e738d", key: "#8aadf4", border: "#494d64", active: "#eed49f"},
	"catppuccin-frappe":    {title: "#ca9ee6", error: "#e78284", muted: "#737994", key: "#8caaee", border: "#51576d", active: "#e5c890"},
	"catppuccin-latte":     {title: "#8839ef", error: "#d20f39", muted: "#8c8fa1", key: "#1e66f5", border: "#ccd0da", active: "#df8e1d"},
}

// applyTheme changes the shared styles to use the requested color theme.
func applyTheme(name string) {
	pal, ok := palettes[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		pal = palettes["default"]
	}
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(pal.title)
	errorStyle = lipgloss.NewStyle().Foreground(pal.error)
	mutedStyle = lipgloss.NewStyle().Foreground(pal.muted)
	keyStyle = lipgloss.NewStyle().Bold(true).Foreground(pal.key)
	panelStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(pal.border).Padding(0, 1)
	activeStyle = lipgloss.NewStyle().Foreground(pal.active)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(pal.active).Background(pal.border)
	syntaxKeywordStyle = lipgloss.NewStyle().Foreground(firstColor(pal.keyword, pal.title))
	syntaxStringStyle = lipgloss.NewStyle().Foreground(firstColor(pal.stringLit, pal.active))
	syntaxCommentStyle = lipgloss.NewStyle().Foreground(firstColor(pal.comment, pal.muted))
	syntaxNumberStyle = lipgloss.NewStyle().Foreground(firstColor(pal.number, pal.key))
}

// firstColor chooses the first configured color, or a readable fallback.
func firstColor(values ...lipgloss.Color) lipgloss.Color {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return lipgloss.Color("255")
}

// themeNames returns sorted theme names for help text and tests.
func themeNames() []string {
	names := make([]string, 0, len(palettes))
	for name := range palettes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
