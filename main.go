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

	reloader := runtimeWorkbenchReloader{config: loadResult.Config}
	data := loadStartupWorkbenchData(context.Background(), startupRepo, reloader)

	_, err = tea.NewProgram(app.NewWithWorkbenchData(loadResult.Config, startupRepo.Repo, data), tea.WithAltScreen()).Run()
	return err
}

type runtimeWorkbenchReloader struct {
	config config.Config
}

func (r runtimeWorkbenchReloader) Load(ctx context.Context, repo workbench.RepoRef) workbench.RuntimeLoadResult {
	resolvedPath := config.ResolveRepositoryPath(config.RepositoryPathOptions{
		Repo:   repo.FullName(),
		Config: r.config,
	})
	if resolvedPath.Path == "" {
		return workbench.RuntimeLoadResult{Repo: repo}
	}
	return (workbench.RuntimeLoader{
		Repo:     repo,
		RepoPath: resolvedPath.Path,
		Local:    localrepo.Service{},
		GitHub:   github.CLIService{},
	}).Load(ctx)
}

func loadStartupWorkbenchData(ctx context.Context, startupRepo config.StartupRepository, reloader app.WorkbenchReloader) app.WorkbenchData {
	repo, ok := repoRefFromFullName(startupRepo.Repo)
	if !ok {
		return app.WorkbenchData{}
	}

	data := app.WorkbenchData{
		Repos:    []workbench.RepoRef{repo},
		Reloader: reloader,
	}
	if reloader == nil {
		return data
	}

	result := reloader.Load(ctx, repo)
	data.WorkItems = result.Items
	return data
}

func repoRefFromFullName(repoName string) (workbench.RepoRef, bool) {
	owner, name, ok := strings.Cut(repoName, "/")
	if !ok || owner == "" || name == "" {
		return workbench.RepoRef{}, false
	}
	return workbench.RepoRef{Owner: owner, Name: name}, true
}

func sameRepoFullName(left string, right string) bool {
	return strings.EqualFold(left, right)
}
