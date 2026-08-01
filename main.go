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

	reloader := runtimeWorkbenchReloader{
		config:      loadResult.Config,
		startupRepo: startupRepo.Repo,
	}
	data := loadStartupWorkbenchData(startupRepo, reloader)

	_, err = tea.NewProgram(app.NewWithWorkbenchData(loadResult.Config, startupRepo.Repo, data), tea.WithAltScreen()).Run()
	return err
}

type runtimeWorkbenchReloader struct {
	config      config.Config
	startupRepo string
	local       localrepo.Service
	github      workbench.GitHubWorkbenchDiscovery
}

func (r runtimeWorkbenchReloader) Load(ctx context.Context, repo workbench.RepoRef) workbench.RuntimeLoadResult {
	local := r.local
	rootPaths, rootDiagnostics := resolvedRepositoryRootPaths(r.config)
	discovered, discoveryDiagnostics := local.DiscoverRepositories(ctx, rootPaths)

	checkouts, diagnostics := r.repositoryCheckouts(ctx, repo, discovered)
	diagnostics = append(diagnostics, repositoryDiagnosticsFromConfig(rootDiagnostics)...)
	diagnostics = append(diagnostics, repositoryDiagnosticsFromLocal(discoveryDiagnostics)...)

	items := []workbench.WorkItem{}
	summaries := make([]workbench.RepositorySummary, 0, len(checkouts))
	rawResultRepo, hasRawResultRepo := r.requestedRepository(repo)
	selectedRawResult := workbench.RuntimeLoadResult{}
	for _, checkout := range checkouts {
		if checkout.path == "" {
			items = append(items, workbench.RepositoryPathErrorItem(checkout.repo, checkout.diagnostics))
			summaries = append(summaries, workbench.RepositorySummary{Repo: checkout.repo})
			continue
		}
		result := (workbench.RuntimeLoader{
			Repo:     checkout.repo,
			RepoPath: checkout.path,
			Local:    local,
			GitHub:   r.githubDiscovery(),
		}).Load(ctx)
		if shouldUseRawResult(checkout.repo, rawResultRepo, hasRawResultRepo, selectedRawResult) {
			selectedRawResult = result
		}
		items = append(items, result.Items...)
		if len(checkout.diagnostics) > 0 {
			items = append(items, workbench.RepositoryPathErrorItem(checkout.repo, checkout.diagnostics))
		}
		summaries = append(summaries, workbench.SummarizeRepository(
			checkout.repo,
			checkout.path,
			checkout.defaultBranch,
			checkout.remotes,
			result.Items,
		))
	}
	if len(diagnostics) > 0 {
		diagnosticRepo := repositoryDiagnosticRepo(repo, summaries, r.startupRepo)
		if len(summaries) == 0 {
			summaries = append(summaries, workbench.RepositorySummary{Repo: diagnosticRepo})
		}
		items = append(items, workbench.RepositoryPathErrorItem(diagnosticRepo, diagnostics))
	}

	return workbench.RuntimeLoadResult{
		Repo:               repo,
		Repositories:       summaries,
		Items:              items,
		PullRequests:       selectedRawResult.PullRequests,
		PullRequestsLoaded: selectedRawResult.PullRequestsLoaded,
		IssuesRepo:         selectedRawResult.IssuesRepo,
		Issues:             selectedRawResult.Issues,
		IssuesLoaded:       selectedRawResult.IssuesLoaded,
		IssuesError:        selectedRawResult.IssuesError,
		ViewerSubject:      selectedRawResult.ViewerSubject,
	}
}

func shouldUseRawResult(checkoutRepo workbench.RepoRef, rawResultRepo workbench.RepoRef, hasRawResultRepo bool, current workbench.RuntimeLoadResult) bool {
	if hasRawResultRepo {
		return repoFullNameKey(checkoutRepo) == repoFullNameKey(rawResultRepo)
	}
	return current.Repo == (workbench.RepoRef{})
}

func loadStartupWorkbenchData(startupRepo config.StartupRepository, reloader app.WorkbenchReloader) app.WorkbenchData {
	data := app.WorkbenchData{
		Reloader:       reloader,
		ActionsLoader:  app.NewGitHubActionsLoader(github.CLIService{}),
		InitialLoading: reloader != nil,
	}
	if issueReloader, ok := reloader.(app.IssueReloader); ok {
		data.IssueReloader = issueReloader
	}
	repo, ok := repoRefFromFullName(startupRepo.Repo)
	if !ok {
		return data
	}

	data.Repos = []workbench.RepoRef{repo}
	return data
}

func (r runtimeWorkbenchReloader) LoadIssues(ctx context.Context, repo workbench.RepoRef) workbench.RuntimeLoadResult {
	checkout := r.checkoutForRepository(ctx, repo)
	if checkout.path == "" {
		return workbench.RuntimeLoadResult{
			Repo:  repo,
			Items: []workbench.WorkItem{workbench.RepositoryPathErrorItem(repo, checkout.diagnostics)},
		}
	}
	result := (workbench.RuntimeLoader{
		Repo:     checkout.repo,
		RepoPath: checkout.path,
		Local:    r.local,
		GitHub:   r.githubDiscovery(),
	}).Load(ctx)
	if len(checkout.diagnostics) > 0 {
		result.Items = append(result.Items, workbench.RepositoryPathErrorItem(checkout.repo, checkout.diagnostics))
	}
	result.Repositories = []workbench.RepositorySummary{workbench.SummarizeRepository(
		checkout.repo,
		checkout.path,
		checkout.defaultBranch,
		checkout.remotes,
		result.Items,
	)}
	return result
}

type repositoryCheckout struct {
	repo          workbench.RepoRef
	path          string
	defaultBranch string
	remotes       []string
	diagnostics   []workbench.RepositoryDiagnostic
}

func (r runtimeWorkbenchReloader) repositoryCheckouts(ctx context.Context, selected workbench.RepoRef, discovered []localrepo.Repository) ([]repositoryCheckout, []workbench.RepositoryDiagnostic) {
	checkouts := make([]repositoryCheckout, 0, len(discovered)+1)
	diagnostics := []workbench.RepositoryDiagnostic{}
	seen := map[string]struct{}{}
	for _, repository := range discovered {
		repoName, err := config.ParseGitHubRemoteURL(repository.OriginURL)
		if err != nil {
			diagnostics = append(diagnostics, workbench.RepositoryDiagnostic{
				Path:    repository.Path,
				Message: err.Error(),
			})
			continue
		}
		repo, ok := repoRefFromFullName(repoName)
		if !ok {
			continue
		}
		key := repoFullNameKey(repo)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		checkouts = append(checkouts, repositoryCheckout{
			repo:          repo,
			path:          repository.Path,
			defaultBranch: repository.DefaultBranch,
			remotes:       repository.Remotes,
		})
	}
	requested, ok := r.requestedRepository(selected)
	if !ok {
		return checkouts, diagnostics
	}
	if _, ok := seen[repoFullNameKey(requested)]; ok {
		return checkouts, diagnostics
	}
	checkout := r.checkoutForRepository(ctx, requested)
	return append([]repositoryCheckout{checkout}, checkouts...), diagnostics
}

func (r runtimeWorkbenchReloader) checkoutForRepository(ctx context.Context, repo workbench.RepoRef) repositoryCheckout {
	resolvedPath := config.ResolveRepositoryPath(config.RepositoryPathOptions{
		Repo:   repo.FullName(),
		Config: r.config,
	})
	if resolvedPath.Path == "" {
		return repositoryCheckout{
			repo:        repo,
			diagnostics: repositoryDiagnosticsFromConfig(resolvedPath.Diagnostics),
		}
	}
	metadata, err := r.local.RepositoryMetadata(ctx, resolvedPath.Path)
	if err != nil {
		return repositoryCheckout{
			repo: repo,
			path: resolvedPath.Path,
			diagnostics: []workbench.RepositoryDiagnostic{{
				Path:    resolvedPath.Path,
				Message: err.Error(),
			}},
		}
	}
	return repositoryCheckout{
		repo:          repo,
		path:          resolvedPath.Path,
		defaultBranch: metadata.DefaultBranch,
		remotes:       metadata.Remotes,
	}
}

func (r runtimeWorkbenchReloader) requestedRepository(selected workbench.RepoRef) (workbench.RepoRef, bool) {
	if selected != (workbench.RepoRef{}) {
		return selected, true
	}
	return repoRefFromFullName(r.startupRepo)
}

func (r runtimeWorkbenchReloader) githubDiscovery() workbench.GitHubWorkbenchDiscovery {
	if r.github != nil {
		return r.github
	}
	return github.CLIService{}
}

func resolvedRepositoryRootPaths(cfg config.Config) ([]string, []config.Diagnostic) {
	roots, diagnostics := config.ResolveRepositoryRoots(cfg)
	paths := make([]string, 0, len(roots))
	for _, root := range roots {
		paths = append(paths, root.Path)
	}
	return paths, diagnostics
}

func repositoryDiagnosticsFromConfig(diagnostics []config.Diagnostic) []workbench.RepositoryDiagnostic {
	out := make([]workbench.RepositoryDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, workbench.RepositoryDiagnostic{
			Path:    diagnostic.Path,
			Message: diagnostic.Message,
		})
	}
	return out
}

func repositoryDiagnosticsFromLocal(diagnostics []localrepo.RepositoryDiagnostic) []workbench.RepositoryDiagnostic {
	out := make([]workbench.RepositoryDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, workbench.RepositoryDiagnostic{
			Path:    diagnostic.Path,
			Message: diagnostic.Message,
		})
	}
	return out
}

func repositoryDiagnosticRepo(selected workbench.RepoRef, summaries []workbench.RepositorySummary, startupRepo string) workbench.RepoRef {
	if selected != (workbench.RepoRef{}) {
		return selected
	}
	if repo, ok := repoRefFromFullName(startupRepo); ok {
		return repo
	}
	if len(summaries) > 0 {
		return summaries[0].Repo
	}
	return workbench.RepoRef{Owner: "local", Name: "repositories"}
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

func repoFullNameKey(repo workbench.RepoRef) string {
	return strings.ToLower(repo.FullName())
}
