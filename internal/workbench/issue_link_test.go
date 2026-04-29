package workbench

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeIssueCheckDiscovery struct {
	issues      []IssueRef
	issueErr    error
	checks      CheckSummary
	checksByRef map[string]CheckSummary
	checkErr    error
	checkErrs   map[string]error
	err         error
}

func (f fakeIssueCheckDiscovery) Issues(context.Context, string) ([]IssueRef, error) {
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.issues, nil
}

func (f fakeIssueCheckDiscovery) CheckSummary(_ context.Context, _ string, ref string) (CheckSummary, error) {
	if err := f.checkErrs[ref]; err != nil {
		return CheckSummary{}, err
	}
	if f.checkErr != nil {
		return CheckSummary{}, f.checkErr
	}
	if f.err != nil {
		return CheckSummary{}, f.err
	}
	if checks, ok := f.checksByRef[ref]; ok {
		return checks, nil
	}
	return f.checks, nil
}

func TestInferIssueFromBranch(t *testing.T) {
	cases := []struct {
		name    string
		branch  string
		number  int
		certain bool
		ok      bool
	}{
		{name: "issue prefix", branch: "feature/issue-123-config", number: 123, certain: true, ok: true},
		{name: "gh prefix", branch: "gh-42", number: 42, certain: true, ok: true},
		{name: "bare number uncertain", branch: "feature/123-config", number: 123, certain: false, ok: true},
		{name: "multiple bare numbers absent", branch: "feature/123-and-456", ok: false},
		{name: "multiple prefixed uncertain", branch: "issue-123-fix-456", number: 123, certain: false, ok: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := InferIssueFromBranch(tc.branch)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v, got %v for %+v", tc.ok, ok, got)
			}
			if !ok {
				return
			}
			if got.Number != tc.number || got.Certain != tc.certain {
				t.Fatalf("expected issue #%d certain=%v, got %+v", tc.number, tc.certain, got)
			}
		})
	}
}

func TestLinkIssues_UsesPRMetadataBeforeBranchHeuristic(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := []WorkItem{{
		ID:     "feature",
		Repo:   repo,
		Branch: &BranchRef{Name: "feature/issue-123-config"},
		PullRequest: &PullRequestRef{
			Number: 24,
			LinkedIssues: []IssueRef{
				{Number: 10, Title: "Config discovery", State: "open", URL: "https://example.test/issues/10", Certain: true},
			},
		},
	}}

	got := LinkIssues(items, []IssueRef{
		{Number: 10, Title: "Config discovery", State: "open", URL: "https://example.test/issues/10", Certain: true},
		{Number: 123, Title: "Branch issue", State: "open", URL: "https://example.test/issues/123", Certain: true},
	})
	if got[0].Issue == nil || got[0].Issue.Number != 10 || !got[0].Issue.Certain {
		t.Fatalf("expected PR metadata issue to win, got %+v", got[0].Issue)
	}
}

func TestLinkIssues_EnrichesBranchHeuristicFromIssueList(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := []WorkItem{{
		ID:     "feature",
		Repo:   repo,
		Branch: &BranchRef{Name: "feature/123-config"},
	}}

	got := LinkIssues(items, []IssueRef{
		{Number: 123, Title: "Config discovery", State: "open", URL: "https://example.test/issues/123", Certain: true},
	})
	if got[0].Issue == nil || got[0].Issue.Title != "Config discovery" || got[0].Issue.Certain {
		t.Fatalf("expected enriched uncertain issue, got %+v", got[0].Issue)
	}
}

func TestIssueCheckLinkService_AddsChecks(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	service := IssueCheckLinkService{
		GitHub: fakeIssueCheckDiscovery{
			issues: []IssueRef{{Number: 123, Title: "Config discovery", State: "open", URL: "https://example.test/issues/123", Certain: true}},
			checks: CheckSummary{State: CheckPassing, Passing: 2},
		},
	}

	got := service.LinkForRepo(context.Background(), repo, []WorkItem{{
		ID:          "feature",
		Repo:        repo,
		Branch:      &BranchRef{Name: "feature/issue-123-config"},
		PullRequest: &PullRequestRef{Number: 24, HeadBranch: "feature/issue-123-config"},
	}})
	if got[0].Issue == nil || got[0].Issue.Number != 123 || !got[0].Issue.Certain {
		t.Fatalf("expected linked issue, got %+v", got[0].Issue)
	}
	if got[0].Checks.State != CheckPassing || got[0].Checks.Passing != 2 {
		t.Fatalf("expected check summary, got %+v", got[0].Checks)
	}
}

func TestIssueCheckLinkService_ContinuesWhenIssueDiscoveryFails(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	service := IssueCheckLinkService{
		GitHub: fakeIssueCheckDiscovery{
			issueErr: errors.New("issues failed"),
			checks:   CheckSummary{State: CheckPassing, Passing: 1},
		},
	}

	got := service.LinkForRepo(context.Background(), repo, []WorkItem{{
		ID:     "feature",
		Repo:   repo,
		Branch: &BranchRef{Name: "feature/issue-123-config"},
		PullRequest: &PullRequestRef{
			Number:     24,
			HeadBranch: "feature/issue-123-config",
			LinkedIssues: []IssueRef{
				{Number: 123, Title: "Config discovery", State: "open", URL: "https://example.test/issues/123", Certain: true},
			},
		},
	}})
	if len(got) != 2 {
		t.Fatalf("expected enriched item plus error item, got %+v", got)
	}
	if got[0].Issue == nil || got[0].Issue.Number != 123 {
		t.Fatalf("expected PR metadata issue despite issue discovery failure, got %+v", got[0].Issue)
	}
	if got[0].Checks.State != CheckPassing || got[0].Checks.Passing != 1 {
		t.Fatalf("expected checks to continue after issue discovery failure, got %+v", got[0].Checks)
	}
	if got[1].Local == nil || !strings.Contains(got[1].Local.Summary, "issues failed") {
		t.Fatalf("expected issue discovery error summary, got %+v", got[1].Local)
	}
}

func TestIssueCheckLinkService_ContinuesWhenSingleCheckFails(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	service := IssueCheckLinkService{
		GitHub: fakeIssueCheckDiscovery{
			checksByRef: map[string]CheckSummary{
				"second": {State: CheckPassing, Passing: 2},
			},
			checkErrs: map[string]error{
				"first": errors.New("first checks failed"),
			},
		},
	}

	got := service.LinkForRepo(context.Background(), repo, []WorkItem{
		{ID: "first", Repo: repo, Branch: &BranchRef{Name: "first"}, PullRequest: &PullRequestRef{Number: 1, HeadBranch: "first"}},
		{ID: "second", Repo: repo, Branch: &BranchRef{Name: "second"}, PullRequest: &PullRequestRef{Number: 2, HeadBranch: "second"}},
	})
	if len(got) != 3 {
		t.Fatalf("expected two items plus one error item, got %+v", got)
	}
	if got[0].Checks.State != CheckUnknown {
		t.Fatalf("expected failed check item to remain unknown, got %+v", got[0].Checks)
	}
	if got[1].Checks.State != CheckPassing || got[1].Checks.Passing != 2 {
		t.Fatalf("expected later PR checks to be linked, got %+v", got[1].Checks)
	}
	if got[2].Local == nil || !strings.Contains(got[2].Local.Summary, "first checks failed") {
		t.Fatalf("expected check discovery error summary, got %+v", got[2].Local)
	}
}

func TestIssueCheckLinkService_ReturnsErrorItem(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	service := IssueCheckLinkService{
		GitHub: fakeIssueCheckDiscovery{err: errors.New("network failed")},
	}

	got := service.LinkForRepo(context.Background(), repo, []WorkItem{{ID: "feature", Repo: repo}})
	if len(got) != 2 {
		t.Fatalf("expected original item plus error item, got %+v", got)
	}
	if got[1].Title() != "issue and check discovery error" {
		t.Fatalf("expected error item title, got %q", got[1].Title())
	}
	if got[1].Local == nil || !strings.Contains(got[1].Local.Summary, "network failed") {
		t.Fatalf("expected error summary, got %+v", got[1].Local)
	}
}
