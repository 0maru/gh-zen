package workbench

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakePullRequestDiscovery struct {
	prs []PullRequestRef
	err error
}

func (f fakePullRequestDiscovery) PullRequests(context.Context, string) ([]PullRequestRef, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.prs, nil
}

func TestLinkPullRequests_MatchesBranchBackedItems(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := []WorkItem{
		{ID: "main", Repo: repo, Branch: &BranchRef{Name: "main"}},
		{ID: "feature", Repo: repo, Branch: &BranchRef{Name: "feature"}},
		{ID: "issue-only", Repo: repo, Issue: &IssueRef{Number: 9, Certain: true}},
	}
	prs := []PullRequestRef{
		{Number: 12, Title: "Add feature", State: "open", URL: "https://example.test/pr/12", HeadOwner: "0maru", HeadBranch: "feature", ReviewState: "review requested"},
	}

	got := LinkPullRequests(items, prs)
	if got[1].PullRequest == nil {
		t.Fatalf("expected feature work item to link PR, got %+v", got[1])
	}
	if got[1].PullRequest.Number != 12 || got[1].PullRequest.ReviewState != "review requested" {
		t.Fatalf("unexpected linked PR: %+v", got[1].PullRequest)
	}
	if got[1].Checks.State != CheckUnknown {
		t.Fatalf("expected check placeholder, got %+v", got[1].Checks)
	}
	if got[0].PullRequest != nil || got[2].PullRequest != nil {
		t.Fatalf("expected unmatched work items to remain without PRs, got %+v", got)
	}
	if items[1].PullRequest != nil {
		t.Fatalf("expected input work items to remain unchanged")
	}
}

func TestLinkPullRequests_ClearsStaleBranchLink(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	stale := PullRequestRef{Number: 9, HeadOwner: "0maru", HeadBranch: "feature"}
	items := []WorkItem{
		{ID: "feature", Repo: repo, Branch: &BranchRef{Name: "feature"}, PullRequest: &stale},
	}

	got := LinkPullRequests(items, nil)
	if got[0].PullRequest != nil {
		t.Fatalf("expected stale PR link to be cleared, got %+v", got[0].PullRequest)
	}
	if items[0].PullRequest == nil {
		t.Fatal("expected input work item to remain unchanged")
	}
}

func TestLinkPullRequests_MatchesHeadOwner(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := []WorkItem{
		{ID: "main", Repo: repo, Branch: &BranchRef{Name: "main"}},
	}
	prs := []PullRequestRef{
		{Number: 1, State: "open", HeadOwner: "fork", HeadBranch: "main"},
		{Number: 2, State: "open", HeadOwner: "0maru", HeadBranch: "main"},
	}

	got := LinkPullRequests(items, prs)
	if got[0].PullRequest == nil || got[0].PullRequest.Number != 2 {
		t.Fatalf("expected same-owner PR link, got %+v", got[0].PullRequest)
	}

	got = LinkPullRequests(items, prs[:1])
	if got[0].PullRequest != nil {
		t.Fatalf("expected fork PR with same branch name to be ignored, got %+v", got[0].PullRequest)
	}
}

func TestLinkPullRequests_PrefersOpenPullRequestForSameHead(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := []WorkItem{
		{ID: "feature", Repo: repo, Branch: &BranchRef{Name: "feature"}},
	}
	prs := []PullRequestRef{
		{Number: 1, State: "closed", HeadOwner: "0maru", HeadBranch: "feature"},
		{Number: 2, State: "open", HeadOwner: "0maru", HeadBranch: "feature"},
	}

	got := LinkPullRequests(items, prs)
	if got[0].PullRequest == nil || got[0].PullRequest.Number != 2 {
		t.Fatalf("expected open PR link, got %+v", got[0].PullRequest)
	}
}

func TestPullRequestLinkService_LoadsPullRequestsForRepo(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	service := PullRequestLinkService{
		GitHub: fakePullRequestDiscovery{
			prs: []PullRequestRef{{Number: 24, HeadOwner: "0maru", HeadBranch: "feature", State: "open"}},
		},
	}

	got := service.LinkForRepo(context.Background(), repo, []WorkItem{
		{ID: "feature", Repo: repo, Branch: &BranchRef{Name: "feature"}},
	})
	if got[0].PullRequest == nil || got[0].PullRequest.Number != 24 {
		t.Fatalf("expected PR link from discovery service, got %+v", got)
	}
}

func TestPullRequestLinkService_ReturnsErrorItem(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	service := PullRequestLinkService{
		GitHub: fakePullRequestDiscovery{err: errors.New("auth failed")},
	}

	got := service.LinkForRepo(context.Background(), repo, []WorkItem{
		{ID: "feature", Repo: repo, Branch: &BranchRef{Name: "feature"}},
	})
	if len(got) != 2 {
		t.Fatalf("expected original item plus error item, got %+v", got)
	}
	if got[1].Title() != "pull request discovery error" {
		t.Fatalf("expected error item title, got %q", got[1].Title())
	}
	if got[1].Local == nil || !strings.Contains(got[1].Local.Summary, "auth failed") {
		t.Fatalf("expected non-fatal error summary, got %+v", got[1].Local)
	}
}
