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

	review           viewport.Model
	commit           textinput.Model
	branchInput      textinput.Model
	fileFilter       textinput.Model
	searchInput      textinput.Model
	setupInputs      []textinput.Model
	setupStep        int
	setupStage       string
	spinner          spinner.Model
	loading          bool
	info             git.RepoInfo
	files            []git.FileStatus
	branches         []string
	fileCursor       int
	fileOffset       int
	branchCursor     int
	branchOffset     int
	profileCursor    int
	profileOffset    int
	conflicts        []string
	conflictCursor   int
	conflictOffset   int
	stashes          []git.Stash
	stashCursor      int
	stashOffset      int
	commits          []git.Commit
	squashMark       []bool
	squashBase       int
	squashCursor     int
	squashOffset     int
	viewerContent    string
	fileFilterActive bool
	searchMatches    []searchMatch
	searchCursor     int
	searchOffset     int
	searchReturn     string
	searchYOffset    int
	target           string
	mode             string
	err              error
	notice           string
	gitOutput        string
	outputExpanded   bool
	footerHidden     bool
	splashFrame      int
	splashMessage    string
}

// Messages Sent Back To Update

// repoLoadedMsg means repo info, file status, and viewer text finished loading.
type repoLoadedMsg struct {
	info   git.RepoInfo
	files  []git.FileStatus
	review string
	target string
	mode   string
	notice string
	err    error
}

// gitActionFinishedMsg means a Git command finished with output or an error.
type gitActionFinishedMsg struct {
	output  string
	refresh bool
	err     error
}

// pushFinishedMsg means a push attempt finished. rejected is true when the
// remote refused it as a non-fast-forward, so a force push could resolve it.
type pushFinishedMsg struct {
	output   string
	rejected bool
	err      error
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

// conflictsLoadedMsg means conflict files and the selected preview are ready.
type conflictsLoadedMsg struct {
	info         git.RepoInfo
	files        []git.FileStatus
	conflicts    []string
	markerReport string
	review       string
	err          error
}

// stashesLoadedMsg means stash entries and the selected diff are ready.
type stashesLoadedMsg struct {
	info    git.RepoInfo
	files   []git.FileStatus
	stashes []git.Stash
	review  string
	notice  string
	err     error
}

// squashLoadedMsg means the commits ahead of the default branch are ready for
// the in-TUI squash picker.
type squashLoadedMsg struct {
	info    git.RepoInfo
	files   []git.FileStatus
	commits []git.Commit
	err     error
}

type setupFinishedMsg struct {
	output string
	err    error
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

	fileFilter := textinput.New()
	fileFilter.Placeholder = "filter changed files"
	fileFilter.CharLimit = 160
	fileFilter.Prompt = "/ "

	searchInput := textinput.New()
	searchInput.Placeholder = "fuzzy find in viewed file"
	searchInput.CharLimit = 160
	searchInput.Prompt = "/ "

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = keyStyle

	model := Model{
		runner:        runner,
		config:        cfg,
		review:        viewport.New(80, 24),
		commit:        commit,
		branchInput:   branchInput,
		fileFilter:    fileFilter,
		searchInput:   searchInput,
		spinner:       sp,
		squashBase:    -1,
		target:        "repo",
		mode:          "review",
		splash:        true,
		splashMessage: randomSplashMessage(),
	}
	if config.NeedsFirstRunSetup(cfg) {
		model.splash = false
		model.mode = "setup"
		model.setupStage = "welcome"
		model.setupInputs = newSetupInputs(runner, cfg)
	}
	return model
}

// Init starts the splash timer; loading the repository begins after the splash.
func (m Model) Init() tea.Cmd {
	if m.mode == "setup" {
		return nil
	}
	return tickSplash()
}
