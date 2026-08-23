package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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

	review               viewport.Model
	commit               textinput.Model
	branchInput          textinput.Model
	fileFilter           textinput.Model
	searchInput          textinput.Model
	commandInput         textinput.Model
	setupInputs          []textinput.Model
	setupStep            int
	setupStage           string
	projects             []string
	projectCursor        int
	projectInput         textinput.Model
	projectAction        string
	spinner              spinner.Model
	loading              bool
	info                 git.RepoInfo
	files                []git.FileStatus
	branches             []string
	fileCursor           int
	fileOffset           int
	branchCursor         int
	branchOffset         int
	profileCursor        int
	profileOffset        int
	conflicts            []string
	conflictCursor       int
	conflictOffset       int
	stashes              []git.Stash
	releases             []git.Release
	releaseDetail        git.ReleaseDetail
	releaseCursor        int
	releaseInput         int
	releaseStep          int
	releaseInputs        []textinput.Model
	releaseNotes         textarea.Model
	releaseProgress      progress.Model
	releaseDraft         bool
	releasePrerelease    bool
	releaseGenerateNotes bool
	stashCursor          int
	stashOffset          int
	commits              []git.Commit
	squashMark           []bool
	squashBase           int
	squashCursor         int
	squashOffset         int
	viewerContent        string
	fileFilterActive     bool
	searchMatches        []searchMatch
	searchCursor         int
	searchOffset         int
	searchReturn         string
	searchYOffset        int
	target               string
	mode                 string
	helpReturn           string
	returnMode           string
	themeCursor          int
	commandCursor        int
	toast                string
	toastError           bool
	err                  error
	notice               string
	gitOutput            string
	outputExpanded       bool
	footerHidden         bool
	splashFrame          int
	splashMessage        string
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

type projectsLoadedMsg struct {
	projects []string
	err      error
}

type projectOpenedMsg struct {
	path   string
	output string
	err    error
}

type releasesLoadedMsg struct {
	releases []git.Release
	err      error
}

type releaseCreatedMsg struct {
	output string
	err    error
}

type releaseDetailLoadedMsg struct {
	detail git.ReleaseDetail
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

	commandInput := textinput.New()
	commandInput.Placeholder = "Type a command"
	commandInput.CharLimit = 120
	commandInput.Prompt = "> "

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = keyStyle

	releaseNotes := textarea.New()
	releaseNotes.Placeholder = "What changed in this release? Markdown is welcome."
	releaseNotes.CharLimit = 12000
	releaseNotes.SetWidth(64)
	releaseNotes.SetHeight(8)
	releaseNotes.ShowLineNumbers = false

	releaseProgress := progress.New(progress.WithDefaultGradient(), progress.WithoutPercentage(), progress.WithFillCharacters('━', '─'))
	releaseProgress.Width = 52
	releaseProgress.SetSpringOptions(14, 0.82)

	model := Model{
		runner:          runner,
		config:          cfg,
		review:          viewport.New(80, 24),
		commit:          commit,
		branchInput:     branchInput,
		fileFilter:      fileFilter,
		searchInput:     searchInput,
		commandInput:    commandInput,
		spinner:         sp,
		releaseNotes:    releaseNotes,
		releaseProgress: releaseProgress,
		squashBase:      -1,
		target:          "repo",
		mode:            "review",
		splash:          true,
		splashMessage:   randomSplashMessage(),
	}
	model.projectInput = textinput.New()
	model.projectInput.Prompt = "> "
	model.projectInput.CharLimit = 240
	model.releaseInputs = newReleaseInputs()
	if config.NeedsFirstRunSetup(cfg) {
		model.splash = false
		model.mode = "setup"
		model.setupStage = "welcome"
		model.setupInputs = newSetupInputs(runner, cfg)
	} else if !runner.IsRepository(context.Background()) {
		model.mode = "workspace"
	}
	return model
}

// Init starts the splash timer; loading the repository begins after the splash.
func (m Model) Init() tea.Cmd {
	if m.mode == "setup" {
		return nil
	}
	if m.splash {
		return tickSplash()
	}
	if m.mode == "workspace" {
		return loadProjects(m.config.WorkspaceDir)
	}
	return tickSplash()
}
