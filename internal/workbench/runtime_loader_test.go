package workbench

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0maru/gh-zen/internal/localrepo"
)

type fakeRuntimeGitHub struct {
	prs         []PullRequestRef
	prErr       error
	subjects    ReviewSubjects
	subjectErr  error
	issues      []IssueRef
	issueErr    error
	checks      CheckSummary
	checksByRef map[string]CheckSummary
	checkErr    error
	checkErrs   map[string]error
}

type configurableIssueRuntimeGitHub struct {
	fakeRuntimeGitHub
	includeComments []bool
}

func (f *configurableIssueRuntimeGitHub) IssuesWithOptions(_ context.Context, _ string, opts IssueListOptions) ([]IssueRef, error) {
	f.includeComments = append(f.includeComments, opts.IncludeCommentsCount)
	return f.issues, f.issueErr
}

func (f fakeRuntimeGitHub) PullRequests(context.Context, string) ([]PullRequestRef, error) {
	if f.prErr != nil {
		return nil, f.prErr
	}
	return f.prs, nil
}

func (f fakeRuntimeGitHub) ViewerReviewSubjects(context.Context) (ReviewSubjects, error) {
	if f.subjectErr != nil {
		return f.subjects, f.subjectErr
	}
	return f.subjects, nil
}

func (f fakeRuntimeGitHub) Issues(context.Context, string) ([]IssueRef, error) {
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	return f.issues, nil
}

func (f fakeRuntimeGitHub) CheckSummary(_ context.Context, _ string, ref string) (CheckSummary, error) {
	if err := f.checkErrs[ref]; err != nil {
		return CheckSummary{}, err
	}
	if f.checkErr != nil {
		return CheckSummary{}, f.checkErr
	}
	if checks, ok := f.checksByRef[ref]; ok {
		return checks, nil
	}
	return f.checks, nil
}

func TestRuntimeLoader_LoadsLocalItemsAndGitHubEnrichment(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	loader := RuntimeLoader{
		Repo:     repo,
		RepoPath: "/repo",
		Local: fakeLocalDiscovery{
			worktrees: []localrepo.Worktree{{
				Path:   "/repo-feature",
				Branch: "feature/issue-123-runtime",
			}},
		},
		GitHub: fakeRuntimeGitHub{
			prs: []PullRequestRef{{
				Number:     24,
				Title:      "Add runtime pipeline",
				State:      "open",
				URL:        "https://example.test/pull/24",
				HeadOwner:  "0maru",
				HeadBranch: "feature/issue-123-runtime",
				LinkedIssues: []IssueRef{{
					Number:  123,
					Certain: true,
				}},
				ReviewState: "approved",
			}},
			issues: []IssueRef{{
				Number:  123,
				Title:   "Runtime pipeline",
				State:   "open",
				URL:     "https://example.test/issues/123",
				Body:    "Runtime pipeline issue details",
				Labels:  []string{"enhancement"},
				Certain: true,
			}},
			checks: CheckSummary{State: CheckPassing, Passing: 2},
		},
	}

	result := loader.Load(context.Background())

	if result.Repo != repo {
		t.Fatalf("expected repo %+v, got %+v", repo, result.Repo)
	}
	if result.LocalDiscoveryError != "" {
		t.Fatalf("expected local discovery to succeed, got %q", result.LocalDiscoveryError)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one enriched item, got %+v", result.Items)
	}
	if !result.PullRequestsLoaded || len(result.PullRequests) != 1 || result.PullRequests[0].Number != 24 {
		t.Fatalf("expected raw pull requests in result, got loaded=%v prs=%+v", result.PullRequestsLoaded, result.PullRequests)
	}
	if !result.IssuesLoaded || len(result.Issues) != 1 || result.Issues[0].Number != 123 {
		t.Fatalf("expected raw issues in result, got loaded=%v issues=%+v", result.IssuesLoaded, result.Issues)
	}
	if result.IssuesRepo != repo {
		t.Fatalf("expected raw issue source repo %+v, got %+v", repo, result.IssuesRepo)
	}
	item := result.Items[0]
	if item.PullRequest == nil || item.PullRequest.Number != 24 || item.PullRequest.ReviewState != "approved" {
		t.Fatalf("expected linked PR, got %+v", item.PullRequest)
	}
	if item.Issue == nil || item.Issue.Number != 123 || item.Issue.Title != "Runtime pipeline" || item.Issue.Body != "Runtime pipeline issue details" || !item.Issue.Certain {
		t.Fatalf("expected linked issue, got %+v", item.Issue)
	}
	if item.Checks.State != CheckPassing || item.Checks.Passing != 2 {
		t.Fatalf("expected passing checks, got %+v", item.Checks)
	}
}

func TestRuntimeLoader_DefersIssueCommentsUnlessRequested(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	discovery := &configurableIssueRuntimeGitHub{fakeRuntimeGitHub: fakeRuntimeGitHub{
		issues: []IssueRef{{Number: 75, Title: "Issue browsing", State: "open"}},
	}}

	RuntimeLoader{Repo: repo, RepoPath: "/repo", Local: fakeLocalDiscovery{}, GitHub: discovery}.Load(context.Background())
	RuntimeLoader{Repo: repo, RepoPath: "/repo", Local: fakeLocalDiscovery{}, GitHub: discovery, IncludeIssueCommentsCount: true}.Load(context.Background())

	if len(discovery.includeComments) != 2 || discovery.includeComments[0] || !discovery.includeComments[1] {
		t.Fatalf("expected normal load to defer comments and issue-scoped load to include them, got %v", discovery.includeComments)
	}
}

func TestRuntimeLoader_ReturnsViewerSubjectForIssueFilters(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	result := RuntimeLoader{
		Repo:     repo,
		RepoPath: "/repo",
		Local: fakeLocalDiscovery{
			branches: []localrepo.Branch{{Name: "main"}},
		},
		GitHub: fakeRuntimeGitHub{
			subjects: ReviewSubjects{Login: "0maru"},
			prs:      []PullRequestRef{},
			issues:   []IssueRef{},
		},
	}.Load(context.Background())

	if result.ViewerSubject.Login != "0maru" {
		t.Fatalf("expected viewer subject to be returned, got %+v", result.ViewerSubject)
	}
}

func TestRuntimeLoader_ReturnsViewerSubjectWhenPullRequestsFail(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	result := RuntimeLoader{
		Repo:     repo,
		RepoPath: "/repo",
		Local: fakeLocalDiscovery{
			branches: []localrepo.Branch{{Name: "main"}},
		},
		GitHub: fakeRuntimeGitHub{
			prErr:    errors.New("pull requests unavailable"),
			subjects: ReviewSubjects{Login: "0maru"},
			issues:   []IssueRef{{Number: 75, Title: "Issue browsing", State: "open"}},
		},
	}.Load(context.Background())

	if result.ViewerSubject.Login != "0maru" {
		t.Fatalf("expected viewer subject despite pull request failure, got %+v", result.ViewerSubject)
	}
	if !result.IssuesLoaded || len(result.Issues) != 1 || result.Issues[0].Number != 75 {
		t.Fatalf("expected issues to remain available, got loaded=%v issues=%+v", result.IssuesLoaded, result.Issues)
	}
	if !hasRuntimeErrorItem(result.Items, "pull request discovery failed", "pull requests unavailable") {
		t.Fatalf("expected pull request error item, got %+v", result.Items)
	}
}

func TestRuntimeLoader_RecordsViewerSubjectFailure(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	result := RuntimeLoader{
		Repo:     repo,
		RepoPath: "/repo",
		Local: fakeLocalDiscovery{
			branches: []localrepo.Branch{{Name: "feature"}},
		},
		GitHub: fakeRuntimeGitHub{
			subjectErr: errors.New("viewer unavailable"),
			prs: []PullRequestRef{{
				Number:     10,
				State:      "open",
				HeadOwner:  "0maru",
				HeadBranch: "feature",
			}},
			issues: []IssueRef{},
		},
	}.Load(context.Background())

	if !result.PullRequestsLoaded {
		t.Fatalf("expected pull requests to remain loaded, got %+v", result)
	}
	if !strings.Contains(result.ViewerSubjectError, "viewer unavailable") {
		t.Fatalf("expected viewer subject failure to be recorded, got %q", result.ViewerSubjectError)
	}
	if !hasRuntimeErrorItem(result.Items, "pull request discovery failed", "viewer unavailable") {
		t.Fatalf("expected viewer subject error item, got %+v", result.Items)
	}
}

func TestRuntimeLoader_AddsPullRequestBackedItems(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	loader := RuntimeLoader{
		Repo:     repo,
		RepoPath: "/repo",
		Local: fakeLocalDiscovery{
			branches: []localrepo.Branch{{Name: "main"}},
		},
		GitHub: fakeRuntimeGitHub{
			prs: []PullRequestRef{{
				Number:     31,
				Title:      "Review fork work",
				State:      "open",
				URL:        "https://example.test/pull/31",
				HeadOwner:  "contributor",
				HeadBranch: "feature/issue-77-review",
				BaseBranch: "main",
				LinkedIssues: []IssueRef{{
					Number:  77,
					Certain: true,
				}},
			}},
			issues: []IssueRef{{
				Number:  77,
				Title:   "Review queue",
				State:   "open",
				URL:     "https://example.test/issues/77",
				Certain: true,
			}},
			checksByRef: map[string]CheckSummary{
				"31": {State: CheckPassing, Passing: 2},
			},
		},
	}

	result := loader.Load(context.Background())

	item := runtimeWorkItemByPullRequest(result.Items, 31)
	if item == nil {
		t.Fatalf("expected PR-backed item, got %+v", result.Items)
	}
	if item.Worktree != nil {
		t.Fatalf("expected PR-backed item without local worktree, got %+v", item.Worktree)
	}
	if item.Branch == nil || item.Branch.Name != "feature/issue-77-review" || item.Branch.Base != "main" || !item.Branch.RemoteOnly {
		t.Fatalf("expected remote PR branch context, got %+v", item.Branch)
	}
	if item.Location() != "contributor/feature/issue-77-review" {
		t.Fatalf("expected fork head location, got %q", item.Location())
	}
	if item.Issue == nil || item.Issue.Number != 77 || item.Issue.Title != "Review queue" || !item.Issue.Certain {
		t.Fatalf("expected linked issue enrichment, got %+v", item.Issue)
	}
	if item.Checks.State != CheckPassing || item.Checks.Passing != 2 {
		t.Fatalf("expected checks to use PR number for PR-only item, got %+v", item.Checks)
	}
}

func TestRuntimeLoader_PreservesLocalItemsWhenGitHubFails(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	loader := RuntimeLoader{
		Repo:     repo,
		RepoPath: "/repo",
		Local: fakeLocalDiscovery{
			branches: []localrepo.Branch{{Name: "feature/local-only"}},
		},
		GitHub: fakeRuntimeGitHub{
			prErr:    errors.New("gh auth failed"),
			issueErr: errors.New("network failed"),
		},
	}

	result := loader.Load(context.Background())

	if !hasWorkItem(result.Items, func(item WorkItem) bool {
		return item.Branch != nil && item.Branch.Name == "feature/local-only"
	}) {
		t.Fatalf("expected local branch to remain visible, got %+v", result.Items)
	}
	if !hasRuntimeErrorItem(result.Items, "pull request discovery failed", "gh auth failed") {
		t.Fatalf("expected pull request discovery error item, got %+v", result.Items)
	}
	if !hasRuntimeErrorItem(result.Items, "issue and check discovery failed", "network failed") {
		t.Fatalf("expected issue and check discovery error item, got %+v", result.Items)
	}
	if !strings.Contains(result.IssuesError, "network failed") {
		t.Fatalf("expected issue-specific error, got %q", result.IssuesError)
	}
}

func TestRuntimeLoader_PreservesPartialIssuesWhenMetadataFails(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	github := &configurableIssueRuntimeGitHub{fakeRuntimeGitHub: fakeRuntimeGitHub{
		issues:   []IssueRef{{Number: 75, Repository: repo.FullName(), Title: "Issue browser", Certain: true}},
		issueErr: errors.New("comment counts failed"),
	}}
	loader := RuntimeLoader{
		Repo:                      repo,
		RepoPath:                  "/repo",
		Local:                     fakeLocalDiscovery{},
		GitHub:                    github,
		IncludeIssueCommentsCount: true,
	}

	result := loader.Load(context.Background())

	if !result.IssuesLoaded || len(result.Issues) != 1 || result.Issues[0].Number != 75 {
		t.Fatalf("expected partial issues to remain loaded, got %+v", result)
	}
	if !strings.Contains(result.IssuesError, "comment counts failed") {
		t.Fatalf("expected the metadata failure to be reported separately, got %q", result.IssuesError)
	}
}

func TestRuntimeLoader_ReturnsLocalDiscoveryErrorItem(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	loader := RuntimeLoader{
		Repo:     repo,
		RepoPath: "/repo",
		Local:    fakeLocalDiscovery{err: errors.New("git failed")},
	}

	result := loader.Load(context.Background())

	if len(result.Items) != 1 {
		t.Fatalf("expected one local error item, got %+v", result.Items)
	}
	if result.Items[0].Title() != "local discovery error" {
		t.Fatalf("expected local discovery error title, got %q", result.Items[0].Title())
	}
	if result.Items[0].Local == nil || !strings.Contains(result.Items[0].Local.Summary, "git failed") {
		t.Fatalf("expected local discovery error summary, got %+v", result.Items[0].Local)
	}
	if !strings.Contains(result.LocalDiscoveryError, "git failed") {
		t.Fatalf("expected local discovery error to be recorded, got %q", result.LocalDiscoveryError)
	}
}

func TestRuntimeLoader_ContinuesWhenSingleCheckFails(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	loader := RuntimeLoader{
		Repo:     repo,
		RepoPath: "/repo",
		Local: fakeLocalDiscovery{
			branches: []localrepo.Branch{
				{Name: "first"},
				{Name: "second"},
			},
		},
		GitHub: fakeRuntimeGitHub{
			prs: []PullRequestRef{
				{Number: 1, HeadOwner: "0maru", HeadBranch: "first"},
				{Number: 2, HeadOwner: "0maru", HeadBranch: "second"},
			},
			checksByRef: map[string]CheckSummary{
				"second": {State: CheckPassing, Passing: 2},
			},
			checkErrs: map[string]error{
				"first": errors.New("first checks failed"),
			},
		},
	}

	result := loader.Load(context.Background())

	first := runtimeWorkItemByBranch(result.Items, "first")
	if first == nil {
		t.Fatalf("expected first work item, got %+v", result.Items)
	}
	if first.Checks.State != CheckUnknown {
		t.Fatalf("expected failed check item to remain unknown, got %+v", first.Checks)
	}
	second := runtimeWorkItemByBranch(result.Items, "second")
	if second == nil {
		t.Fatalf("expected second work item, got %+v", result.Items)
	}
	if second.Checks.State != CheckPassing || second.Checks.Passing != 2 {
		t.Fatalf("expected later PR checks to be linked, got %+v", second.Checks)
	}
	if len(result.FailedCheckRefs) != 1 || result.FailedCheckRefs[0] != "first" {
		t.Fatalf("expected failed check ref to be recorded, got %+v", result.FailedCheckRefs)
	}
	if !hasRuntimeErrorItem(result.Items, "issue and check discovery failed", "first checks failed") {
		t.Fatalf("expected check discovery error item, got %+v", result.Items)
	}
	if result.IssuesError != "" {
		t.Fatalf("expected check failure not to set issue-specific error, got %q", result.IssuesError)
	}
}

func TestRuntimeLoader_WithTemporaryGitRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	repoDir := initLocalServiceRepo(t)
	featureDir := filepath.Join(t.TempDir(), "feature")
	runLocalServiceGit(t, repoDir, "worktree", "add", "-b", "feature/issue-123-runtime", featureDir)
	writeLocalServiceFile(t, filepath.Join(featureDir, "dirty.txt"), "dirty\n")
	runLocalServiceGit(t, repoDir, "update-ref", "refs/remotes/origin/remote-only", "HEAD")

	result := RuntimeLoader{
		Repo:     repo,
		RepoPath: repoDir,
		Local:    localrepo.Service{},
		GitHub: fakeRuntimeGitHub{
			prs: []PullRequestRef{{
				Number:     24,
				Title:      "Runtime pipeline",
				State:      "open",
				URL:        "https://example.test/pull/24",
				HeadOwner:  "0maru",
				HeadBranch: "feature/issue-123-runtime",
				LinkedIssues: []IssueRef{{
					Number:  123,
					Certain: true,
				}},
			}},
			issues: []IssueRef{{
				Number:  123,
				Title:   "Runtime pipeline",
				State:   "open",
				URL:     "https://example.test/issues/123",
				Certain: true,
			}},
			checks: CheckSummary{State: CheckPassing, Passing: 3},
		},
	}.Load(context.Background())

	if !hasWorkItem(result.Items, func(item WorkItem) bool {
		return item.Worktree != nil &&
			sameRuntimeTestPath(t, item.Worktree.Path, repoDir) &&
			item.Branch != nil &&
			item.Branch.Name == "main" &&
			item.Local != nil &&
			item.Local.State == LocalClean
	}) {
		t.Fatalf("expected clean main worktree item, got %+v", result.Items)
	}
	if !hasWorkItem(result.Items, func(item WorkItem) bool {
		return item.Worktree != nil &&
			sameRuntimeTestPath(t, item.Worktree.Path, featureDir) &&
			item.Branch != nil &&
			item.Branch.Name == "feature/issue-123-runtime" &&
			item.Local != nil &&
			item.Local.State == LocalDirty &&
			item.PullRequest != nil &&
			item.PullRequest.Number == 24 &&
			item.Issue != nil &&
			item.Issue.Number == 123 &&
			item.Checks.State == CheckPassing &&
			item.Checks.Passing == 3
	}) {
		t.Fatalf("expected dirty feature worktree enriched with PR, issue, and checks, got %+v", result.Items)
	}
	if !hasWorkItem(result.Items, func(item WorkItem) bool {
		return item.Worktree == nil &&
			item.Branch != nil &&
			item.Branch.Name == "remote-only" &&
			item.Branch.RemoteOnly &&
			item.Local != nil &&
			item.Local.State == LocalMissing
	}) {
		t.Fatalf("expected remote-only branch item, got %+v", result.Items)
	}
}

func hasRuntimeErrorItem(items []WorkItem, prefix string, detail string) bool {
	return hasWorkItem(items, func(item WorkItem) bool {
		return item.Local != nil &&
			strings.Contains(item.Local.Summary, prefix) &&
			strings.Contains(item.Local.Summary, detail)
	})
}

func runtimeWorkItemByBranch(items []WorkItem, branch string) *WorkItem {
	for i := range items {
		if items[i].Branch != nil && items[i].Branch.Name == branch {
			return &items[i]
		}
	}
	return nil
}

func runtimeWorkItemByPullRequest(items []WorkItem, number int) *WorkItem {
	for i := range items {
		if items[i].PullRequest != nil && items[i].PullRequest.Number == number {
			return &items[i]
		}
	}
	return nil
}

func sameRuntimeTestPath(t *testing.T, got string, want string) bool {
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
