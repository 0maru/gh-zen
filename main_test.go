package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/0maru/gh-zen/internal/config"
	"github.com/0maru/gh-zen/internal/localrepo"
	"github.com/0maru/gh-zen/internal/workbench"
)

type fakeMainGitHub struct {
	prsByRepo    map[string][]workbench.PullRequestRef
	issuesByRepo map[string][]workbench.IssueRef
	checks       map[string]workbench.CheckSummary
	subjects     workbench.ReviewSubjects
	subjectErr   error
	calls        *[]string
}

func (f fakeMainGitHub) PullRequests(_ context.Context, repo string) ([]workbench.PullRequestRef, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, "pull_requests:"+repo)
	}
	return append([]workbench.PullRequestRef(nil), f.prsByRepo[repo]...), nil
}

func (f fakeMainGitHub) Issues(_ context.Context, repo string) ([]workbench.IssueRef, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, "issues:"+repo)
	}
	return append([]workbench.IssueRef(nil), f.issuesByRepo[repo]...), nil
}

func (f fakeMainGitHub) CheckSummary(_ context.Context, repo string, ref string) (workbench.CheckSummary, error) {
	if summary, ok := f.checks[repo+"@"+ref]; ok {
		return summary, nil
	}
	return workbench.CheckSummary{State: workbench.CheckUnknown}, nil
}

func (f fakeMainGitHub) ViewerReviewSubjects(context.Context) (workbench.ReviewSubjects, error) {
	return f.subjects, f.subjectErr
}

func TestRepoRefFromFullName(t *testing.T) {
	got, ok := repoRefFromFullName("0maru/gh-zen")
	if !ok {
		t.Fatalf("expected repo ref to parse")
	}
	if want := (workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}); got != want {
		t.Fatalf("expected %+v, got %+v", want, got)
	}

	if _, ok := repoRefFromFullName(""); ok {
		t.Fatalf("expected empty repo name to be rejected")
	}
}

func TestSameRepoFullName(t *testing.T) {
	if !sameRepoFullName("Owner/Repo", "owner/repo") {
		t.Fatalf("expected repo names to compare case-insensitively")
	}
	if sameRepoFullName("owner/other", "owner/repo") {
		t.Fatalf("expected different repo names not to match")
	}
}

func TestRepositoryCheckoutsDedupesRequestedRepoCaseInsensitively(t *testing.T) {
	selected := workbench.RepoRef{Owner: "owner", Name: "repo"}
	checkouts, diagnostics := (runtimeWorkbenchReloader{config: config.Defaults()}).repositoryCheckouts(
		context.Background(),
		selected,
		[]localrepo.Repository{{
			Path:          "/repos/repo",
			OriginURL:     "https://github.com/Owner/Repo.git",
			DefaultBranch: "main",
			Remotes:       []string{"origin"},
		}},
	)

	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", diagnostics)
	}
	if len(checkouts) != 1 {
		t.Fatalf("expected one checkout, got %+v", checkouts)
	}
	if checkouts[0].path != "/repos/repo" || checkouts[0].repo.FullName() != "Owner/Repo" {
		t.Fatalf("expected discovered checkout only, got %+v", checkouts[0])
	}
}

func TestRuntimeWorkbenchReloaderDiscoversConfiguredRoots(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	root := t.TempDir()
	ghZenPath := filepath.Join(root, "0maru", "gh-zen")
	dotfilesPath := filepath.Join(root, "0maru", "dotfiles")
	initRuntimeRepo(t, ghZenPath, "https://github.com/0maru/gh-zen.git")
	initRuntimeRepo(t, dotfilesPath, "git@github.com:0maru/dotfiles.git")
	featurePath := filepath.Join(t.TempDir(), "gh-zen-feature")
	runRuntimeGit(t, ghZenPath, "worktree", "add", "-b", "feature/issue-74", featurePath)

	cfg := config.Defaults()
	cfg.Repos.Roots = []string{root, filepath.Join(root, "missing")}
	reloader := runtimeWorkbenchReloader{
		config:      cfg,
		startupRepo: "0maru/dotfiles",
		github: fakeMainGitHub{
			prsByRepo: map[string][]workbench.PullRequestRef{
				"0maru/gh-zen": {{
					Number:     74,
					Title:      "Repository root discovery",
					State:      "open",
					HeadOwner:  "0maru",
					HeadBranch: "feature/issue-74",
					LinkedIssues: []workbench.IssueRef{{
						Number:  74,
						Certain: true,
					}},
				}},
			},
			issuesByRepo: map[string][]workbench.IssueRef{
				"0maru/gh-zen": {{
					Number:  74,
					Title:   "Repository root discovery",
					State:   "open",
					Certain: true,
				}},
			},
			checks: map[string]workbench.CheckSummary{
				"0maru/gh-zen@feature/issue-74": {State: workbench.CheckFailing, Failing: 1},
			},
		},
	}

	result := reloader.Load(context.Background(), workbench.RepoRef{Owner: "0maru", Name: "dotfiles"})

	ghZenSummary := requireRepositorySummary(t, result.Repositories, workbench.RepoRef{Owner: "0maru", Name: "gh-zen"})
	if !sameRuntimePath(t, ghZenSummary.Path, ghZenPath) {
		t.Fatalf("expected gh-zen path %q, got %+v", ghZenPath, ghZenSummary)
	}
	if ghZenSummary.DefaultBranch != "main" {
		t.Fatalf("expected default branch main, got %+v", ghZenSummary)
	}
	if len(ghZenSummary.Remotes) != 1 || ghZenSummary.Remotes[0] != "origin" {
		t.Fatalf("expected origin remote, got %+v", ghZenSummary.Remotes)
	}
	if ghZenSummary.ActiveWorktreeCount != 2 {
		t.Fatalf("expected main and feature worktrees to be active, got %+v", ghZenSummary)
	}
	if ghZenSummary.OpenPullRequestCount != 1 || ghZenSummary.OpenIssueCount != 1 || ghZenSummary.FailingCheckCount != 1 {
		t.Fatalf("expected GitHub preview counts, got %+v", ghZenSummary)
	}
	requireRepositorySummary(t, result.Repositories, workbench.RepoRef{Owner: "0maru", Name: "dotfiles"})

	if !hasRuntimeItem(result.Items, func(item workbench.WorkItem) bool {
		return item.Repo.FullName() == "0maru/gh-zen" &&
			item.Branch != nil &&
			item.Branch.Name == "feature/issue-74" &&
			item.PullRequest != nil &&
			item.PullRequest.Number == 74 &&
			item.Checks.State == workbench.CheckFailing
	}) {
		t.Fatalf("expected enriched feature work item, got %+v", result.Items)
	}
	if !hasRuntimeItem(result.Items, func(item workbench.WorkItem) bool {
		return strings.HasPrefix(item.ID, "repository-path-error:") &&
			item.Local != nil &&
			strings.Contains(item.Local.Summary, "repos.roots[1]") &&
			strings.Contains(item.Local.Summary, "not accessible")
	}) {
		t.Fatalf("expected missing root diagnostic item while preserving work, got %+v", result.Items)
	}
}

func TestRuntimeWorkbenchReloaderPropagatesSelectedRepoRawData(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	root := t.TempDir()
	ghZenPath := filepath.Join(root, "0maru", "gh-zen")
	initRuntimeRepo(t, ghZenPath, "https://github.com/0maru/gh-zen.git")

	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	cfg := config.Defaults()
	cfg.Repos.Roots = []string{root}
	reloader := runtimeWorkbenchReloader{
		config: cfg,
		github: fakeMainGitHub{
			prsByRepo: map[string][]workbench.PullRequestRef{
				repo.FullName(): {{
					Number:     81,
					Title:      "Issue browsing",
					State:      "open",
					HeadOwner:  "0maru",
					HeadBranch: "feature/issues",
				}},
			},
			issuesByRepo: map[string][]workbench.IssueRef{
				repo.FullName(): {{
					Number:  123,
					Title:   "Raw issue",
					State:   "open",
					Certain: true,
				}},
			},
			subjects: workbench.ReviewSubjects{Login: "0maru"},
		},
	}

	result := reloader.Load(context.Background(), repo)
	if !result.PullRequestsLoaded || len(result.PullRequests) != 1 || result.PullRequests[0].Number != 81 {
		t.Fatalf("expected selected repo pull requests to propagate, got loaded=%v prs=%+v", result.PullRequestsLoaded, result.PullRequests)
	}
	if !result.IssuesLoaded || len(result.Issues) != 1 || result.Issues[0].Number != 123 {
		t.Fatalf("expected selected repo issues to propagate, got loaded=%v issues=%+v", result.IssuesLoaded, result.Issues)
	}
	if result.IssuesRepo != repo {
		t.Fatalf("expected selected repo issue source %+v, got %+v", repo, result.IssuesRepo)
	}
	if result.ViewerSubject.Login != "0maru" {
		t.Fatalf("expected viewer subject to propagate, got %+v", result.ViewerSubject)
	}
}

func TestRuntimeWorkbenchReloaderPropagatesViewerSubjectError(t *testing.T) {
	if testing.Short() {
		t.Skip("uses a temporary Git repository")
	}

	root := t.TempDir()
	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	initRuntimeRepo(t, filepath.Join(root, repo.Owner, repo.Name), "https://github.com/0maru/gh-zen.git")
	cfg := config.Defaults()
	cfg.Repos.Roots = []string{root}
	reloader := runtimeWorkbenchReloader{
		config: cfg,
		github: fakeMainGitHub{
			prsByRepo:    map[string][]workbench.PullRequestRef{repo.FullName(): {}},
			issuesByRepo: map[string][]workbench.IssueRef{repo.FullName(): {}},
			subjectErr:   errors.New("viewer unavailable"),
		},
	}

	result := reloader.Load(context.Background(), repo)
	if !strings.Contains(result.ViewerSubjectError, "viewer unavailable") {
		t.Fatalf("expected viewer subject failure to propagate, got %q", result.ViewerSubjectError)
	}
}

func TestRuntimeWorkbenchReloaderLoadsIssuesForOnlyRequestedRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	root := t.TempDir()
	ghZenRepo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	dotfilesRepo := workbench.RepoRef{Owner: "0maru", Name: "dotfiles"}
	initRuntimeRepo(t, filepath.Join(root, ghZenRepo.Owner, ghZenRepo.Name), "https://github.com/0maru/gh-zen.git")
	initRuntimeRepo(t, filepath.Join(root, dotfilesRepo.Owner, dotfilesRepo.Name), "https://github.com/0maru/dotfiles.git")

	calls := []string{}
	cfg := config.Defaults()
	cfg.Repos.Roots = []string{root}
	reloader := runtimeWorkbenchReloader{
		config: cfg,
		github: fakeMainGitHub{
			issuesByRepo: map[string][]workbench.IssueRef{
				dotfilesRepo.FullName(): {{Number: 75, Title: "Issue browsing", State: "open"}},
			},
			calls: &calls,
		},
	}

	result := reloader.LoadIssues(context.Background(), dotfilesRepo)

	if !result.IssuesLoaded || len(result.Issues) != 1 || result.Issues[0].Number != 75 {
		t.Fatalf("expected requested repo issues, got loaded=%v issues=%+v", result.IssuesLoaded, result.Issues)
	}
	if len(result.Repositories) != 1 || result.Repositories[0].Repo != dotfilesRepo {
		t.Fatalf("expected one repo-scoped repository summary, got %+v", result.Repositories)
	}
	for _, call := range calls {
		if strings.Contains(call, ghZenRepo.FullName()) {
			t.Fatalf("expected no GitHub calls for unrelated repo, got %+v", calls)
		}
	}
	if !slices.Contains(calls, "pull_requests:"+dotfilesRepo.FullName()) || !slices.Contains(calls, "issues:"+dotfilesRepo.FullName()) {
		t.Fatalf("expected requested repo GitHub calls, got %+v", calls)
	}
}

func TestRuntimeWorkbenchReloaderLoadIssuesReportsMissingCheckout(t *testing.T) {
	repo := workbench.RepoRef{Owner: "0maru", Name: "missing-repo"}
	cfg := config.Defaults()
	cfg.Repos.Roots = []string{t.TempDir()}

	result := (runtimeWorkbenchReloader{config: cfg}).LoadIssues(context.Background(), repo)

	if result.IssuesLoaded {
		t.Fatalf("expected issues not to be loaded, got %+v", result)
	}
	if result.IssuesRepo != repo {
		t.Fatalf("expected issue error source %+v, got %+v", repo, result.IssuesRepo)
	}
	if !strings.Contains(result.IssuesError, "no local checkout found") {
		t.Fatalf("expected checkout diagnostic in issue error, got %q", result.IssuesError)
	}
	if len(result.Items) != 1 || !strings.HasPrefix(result.Items[0].ID, "repository-path-error:") {
		t.Fatalf("expected repository path error item, got %+v", result.Items)
	}
}

func TestRuntimeWorkbenchReloaderPropagatesFirstRepoRawDataForZeroRepoStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	root := t.TempDir()
	ghZenPath := filepath.Join(root, "0maru", "gh-zen")
	initRuntimeRepo(t, ghZenPath, "https://github.com/0maru/gh-zen.git")

	repo := workbench.RepoRef{Owner: "0maru", Name: "gh-zen"}
	cfg := config.Defaults()
	cfg.Repos.Roots = []string{root}
	reloader := runtimeWorkbenchReloader{
		config: cfg,
		github: fakeMainGitHub{
			issuesByRepo: map[string][]workbench.IssueRef{
				repo.FullName(): {{
					Number:  75,
					Title:   "Discovered issue",
					State:   "open",
					Certain: true,
				}},
			},
		},
	}

	result := reloader.Load(context.Background(), workbench.RepoRef{})
	if !result.IssuesLoaded || len(result.Issues) != 1 || result.Issues[0].Number != 75 {
		t.Fatalf("expected first discovered repo issues to propagate, got loaded=%v issues=%+v", result.IssuesLoaded, result.Issues)
	}
	if result.IssuesRepo != repo {
		t.Fatalf("expected first discovered repo issue source %+v, got %+v", repo, result.IssuesRepo)
	}
}

func TestRuntimeWorkbenchReloaderPropagatesRequestedRepoRawDataCaseInsensitively(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	root := t.TempDir()
	repoPath := filepath.Join(root, "Owner", "Repo")
	initRuntimeRepo(t, repoPath, "https://github.com/Owner/Repo.git")

	discoveredRepo := workbench.RepoRef{Owner: "Owner", Name: "Repo"}
	requestedRepo := workbench.RepoRef{Owner: "owner", Name: "repo"}
	cfg := config.Defaults()
	cfg.Repos.Roots = []string{root}
	reloader := runtimeWorkbenchReloader{
		config: cfg,
		github: fakeMainGitHub{
			issuesByRepo: map[string][]workbench.IssueRef{
				discoveredRepo.FullName(): {{
					Number:  82,
					Title:   "Case-preserved issue",
					State:   "open",
					Certain: true,
				}},
			},
		},
	}

	result := reloader.Load(context.Background(), requestedRepo)
	if !result.IssuesLoaded || len(result.Issues) != 1 || result.Issues[0].Number != 82 {
		t.Fatalf("expected requested repo issues to propagate case-insensitively, got loaded=%v issues=%+v", result.IssuesLoaded, result.Issues)
	}
	if result.IssuesRepo != discoveredRepo {
		t.Fatalf("expected discovered repo issue source %+v, got %+v", discoveredRepo, result.IssuesRepo)
	}
}

func requireRepositorySummary(t *testing.T, summaries []workbench.RepositorySummary, repo workbench.RepoRef) workbench.RepositorySummary {
	t.Helper()
	for _, summary := range summaries {
		if summary.Repo == repo {
			return summary
		}
	}
	t.Fatalf("repository summary %+v not found in %+v", repo, summaries)
	return workbench.RepositorySummary{}
}

func hasRuntimeItem(items []workbench.WorkItem, match func(workbench.WorkItem) bool) bool {
	for _, item := range items {
		if match(item) {
			return true
		}
	}
	return false
}

func sameRuntimePath(t *testing.T, got string, want string) bool {
	t.Helper()
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("resolve got path %q: %v", got, err)
	}
	wantResolved, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("resolve want path %q: %v", want, err)
	}
	return gotResolved == wantResolved
}

func initRuntimeRepo(t *testing.T, dir string, origin string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	runRuntimeGit(t, dir, "init", "-b", "main")
	runRuntimeGit(t, dir, "config", "user.email", "test@example.com")
	runRuntimeGit(t, dir, "config", "user.name", "Test User")
	runRuntimeGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runRuntimeGit(t, dir, "add", "README.md")
	runRuntimeGit(t, dir, "commit", "-m", "initial")
	runRuntimeGit(t, dir, "remote", "add", "origin", origin)
}

func runRuntimeGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}
