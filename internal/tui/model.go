package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/config"
	"github.com/b404dev/gitm8/internal/git"
)

// App State

// Model is all state for the terminal UI. Most TUI functions receive a copy of
// Model, change fields on it, and return the changed copy back to Bubble Tea.
type Model struct {
	runner git.Runner
	config config.Config

	width  int
	height int
	ready  bool
	splash bool

	review         viewport.Model
	commit         textinput.Model
	branchInput    textinput.Model
	spinner        spinner.Model
	loading        bool
	info           git.RepoInfo
	files          []git.FileStatus
	branches       []string
	fileCursor     int
	fileOffset     int
	branchCursor   int
	branchOffset   int
	profileCursor  int
	profileOffset  int
	target         string
	mode           string
	err            error
	notice         string
	gitOutput      string
	outputExpanded bool
	footerHidden   bool
	splashFrame    int
	splashMessage  string
}

// Messages Sent Back To Update

// repoLoadedMsg means repo info, file status, and viewer text finished loading.
type repoLoadedMsg struct {
	info   git.RepoInfo
	files  []git.FileStatus
	review string
	target string
	mode   string
	err    error
}

// gitActionFinishedMsg means a Git command finished with output or an error.
type gitActionFinishedMsg struct {
	output  string
	refresh bool
	err     error
}

// commitMessageGeneratedMsg means Codex finished generating a commit subject.
type commitMessageGeneratedMsg struct {
	message string
	err     error
}

// branchesLoadedMsg means the branch picker has fresh branch data.
type branchesLoadedMsg struct {
	info     git.RepoInfo
	files    []git.FileStatus
	branches []string
	mode     string
	err      error
}

// Startup

// New builds the first UI state using the Git runner and loaded config.
func New(runner git.Runner, cfg config.Config) Model {
	applyTheme(cfg.Theme)

	commit := textinput.New()
	commit.Placeholder = "Commit message"
	commit.CharLimit = 200
	commit.Prompt = "> "

	branchInput := textinput.New()
	branchInput.Placeholder = "new-branch-name"
	branchInput.CharLimit = 120
	branchInput.Prompt = "> "

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = keyStyle

	return Model{
		runner:        runner,
		config:        cfg,
		review:        viewport.New(80, 24),
		commit:        commit,
		branchInput:   branchInput,
		spinner:       sp,
		target:        "repo",
		mode:          "review",
		splash:        true,
		splashMessage: randomSplashMessage(),
	}
}

// Init starts the splash timer; loading the repository begins after the splash.
func (m Model) Init() tea.Cmd {
	return tickSplash()
}
