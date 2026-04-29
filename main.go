package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0maru/gh-zen/internal/app"
	"github.com/0maru/gh-zen/internal/config"
	"github.com/0maru/gh-zen/internal/github"
	"github.com/0maru/gh-zen/internal/localrepo"
	"github.com/0maru/gh-zen/internal/workbench"
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
		Config:              loadResult.Config,
		AllowMissingCurrent: true,
	})
	if err != nil {
		return err
	}

	data := loadStartupWorkbenchData(context.Background(), startupRepo)

	_, err = tea.NewProgram(app.NewWithWorkbenchData(loadResult.Config, startupRepo.Repo, data), tea.WithAltScreen()).Run()
	return err
}

func loadStartupWorkbenchData(ctx context.Context, startupRepo config.StartupRepository) app.WorkbenchData {
	repo, ok := repoRefFromFullName(startupRepo.Repo)
	if !ok {
		return app.WorkbenchData{}
	}

	data := app.WorkbenchData{Repos: []workbench.RepoRef{repo}}
	repoPath, ok := currentCheckoutPathForRepo(startupRepo.Repo)
	if !ok {
		return data
	}

	result := (workbench.RuntimeLoader{
		Repo:     repo,
		RepoPath: repoPath,
		Local:    localrepo.Service{},
		GitHub:   github.CLIService{},
	}).Load(ctx)
	data.WorkItems = result.Items
	return data
}

func currentCheckoutPathForRepo(repoName string) (string, bool) {
	currentRepo, err := config.CurrentGitRepository("")
	if err != nil || currentRepo != repoName {
		return "", false
	}
	repoPath, err := config.CurrentGitRepositoryRoot("")
	if err != nil {
		return "", false
	}
	return repoPath, true
}

func repoRefFromFullName(repoName string) (workbench.RepoRef, bool) {
	owner, name, ok := strings.Cut(repoName, "/")
	if !ok || owner == "" || name == "" {
		return workbench.RepoRef{}, false
	}
	return workbench.RepoRef{Owner: owner, Name: name}, true
}
