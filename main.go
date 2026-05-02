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
	data := loadStartupWorkbenchData(startupRepo, reloader)

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
		return workbench.RuntimeLoadResult{
			Repo:  repo,
			Items: []workbench.WorkItem{repositoryPathErrorItem(repo, resolvedPath.Diagnostics)},
		}
	}
	return (workbench.RuntimeLoader{
		Repo:     repo,
		RepoPath: resolvedPath.Path,
		Local:    localrepo.Service{},
		GitHub:   github.CLIService{},
	}).Load(ctx)
}

func loadStartupWorkbenchData(startupRepo config.StartupRepository, reloader app.WorkbenchReloader) app.WorkbenchData {
	repo, ok := repoRefFromFullName(startupRepo.Repo)
	if !ok {
		return app.WorkbenchData{}
	}

	data := app.WorkbenchData{
		Repos:          []workbench.RepoRef{repo},
		Reloader:       reloader,
		InitialLoading: reloader != nil,
	}
	return data
}

func repositoryPathErrorItem(repo workbench.RepoRef, diagnostics []config.Diagnostic) workbench.WorkItem {
	summary := "repository path resolution failed"
	if len(diagnostics) > 0 {
		summary += ": " + diagnosticSummary(diagnostics)
	}
	return workbench.WorkItem{
		ID:     "repository-path-error:" + repo.FullName(),
		Repo:   repo,
		Branch: &workbench.BranchRef{Name: "repository path error"},
		Local: &workbench.LocalStatus{
			State:   workbench.LocalUnknown,
			Summary: summary,
		},
		Checks: workbench.CheckSummary{State: workbench.CheckUnknown},
	}
}

func diagnosticSummary(diagnostics []config.Diagnostic) string {
	parts := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if diagnostic.Path == "" {
			parts = append(parts, diagnostic.Message)
			continue
		}
		parts = append(parts, diagnostic.Path+": "+diagnostic.Message)
	}
	return strings.Join(parts, "; ")
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
