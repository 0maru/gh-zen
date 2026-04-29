package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0maru/gh-zen/internal/app"
	"github.com/0maru/gh-zen/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	loadResult, err := config.Load(config.LoadOptions{})
	if err != nil {
		return err
	}
	startupRepo, err := config.ResolveStartupRepository(config.StartupRepositoryOptions{
		Config: loadResult.Config,
	})
	if err != nil {
		return err
	}

	_, err = tea.NewProgram(app.NewWithConfig(loadResult.Config, startupRepo.Repo), tea.WithAltScreen()).Run()
	return err
}
