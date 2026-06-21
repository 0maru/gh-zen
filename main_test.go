package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0maru/gh-zen/internal/config"
	"github.com/0maru/gh-zen/internal/workbench"
)

type fakeMainGitHub struct {
	prsByRepo    map[string][]workbench.PullRequestRef
	issuesByRepo map[string][]workbench.IssueRef
	checks       map[string]workbench.CheckSummary
}

func (f fakeMainGitHub) PullRequests(_ context.Context, repo string) ([]workbench.PullRequestRef, error) {
	return append([]workbench.PullRequestRef(nil), f.prsByRepo[repo]...), nil
}

func (f fakeMainGitHub) Issues(_ context.Context, repo string) ([]workbench.IssueRef, error) {
	return append([]workbench.IssueRef(nil), f.issuesByRepo[repo]...), nil
}

func (f fakeMainGitHub) CheckSummary(_ context.Context, repo string, ref string) (workbench.CheckSummary, error) {
	if summary, ok := f.checks[repo+"@"+ref]; ok {
		return summary, nil
	}
	return workbench.CheckSummary{State: workbench.CheckUnknown}, nil
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
