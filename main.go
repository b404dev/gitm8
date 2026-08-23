package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/config"
	"github.com/b404dev/gitm8/internal/git"
	"github.com/b404dev/gitm8/internal/logging"
	"github.com/b404dev/gitm8/internal/tui"
)

// main starts the app. It loads config, creates the Git runner, and then hands
// control to the terminal UI.
func main() {
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, "gitm8 is an interactive TUI. Run %q with no arguments.\n", os.Args[0])
		os.Exit(2)
	}

	logEnabled, logLevel, logFile := config.LoadLogSettings()
	_ = logging.Configure(logEnabled, logLevel, logFile)
	cfg := config.Load()
	if err := logging.Configure(cfg.LogEnabled, cfg.LogLevel, cfg.LogFile); err != nil {
		fmt.Fprintf(os.Stderr, "gitm8 logging disabled: %v\n", err)
	}
	defer logging.Close()
	logging.Info("main", "main", "startup",
		logging.F("theme", cfg.Theme),
		logging.F("default_branch", cfg.DefaultBranch),
		logging.F("ai_provider", cfg.AIProvider),
		logging.F("fetch_on_startup", cfg.FetchOnStartup),
		logging.F("log_enabled", cfg.LogEnabled),
		logging.F("log_level", cfg.LogLevel),
		logging.F("log_file", cfg.LogFile),
	)

	runner := git.NewRunner("")
	if cfg.FetchOnStartup {
		logging.Info("main", "main", "startup_fetch_begin")
		if err := runner.Fetch(context.Background()); err != nil {
			logging.Warn("main", "main", "startup_fetch_failed", logging.F("error", err))
		} else {
			logging.Info("main", "main", "startup_fetch_complete")
		}
	}

	program := tea.NewProgram(tui.New(runner, cfg), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		logging.Error("main", "main", "program_failed", logging.F("error", err))
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	logging.Info("main", "main", "shutdown")
}
