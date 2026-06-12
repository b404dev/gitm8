package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/b404dev/gitm8/internal/config"
	"github.com/b404dev/gitm8/internal/git"
	"github.com/b404dev/gitm8/internal/tui"
)

// main starts the app. It loads config, creates the Git runner, and then hands
// control to the terminal UI.
func main() {
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, "gitm8 is an interactive TUI. Run %q with no arguments.\n", os.Args[0])
		os.Exit(2)
	}

	cfg := config.Load()
	runner := git.NewRunner("")
	if cfg.FetchOnStartup {
		_ = runner.Fetch(context.Background())
	}

	program := tea.NewProgram(tui.New(runner, cfg), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
