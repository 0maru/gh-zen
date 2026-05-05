package workbench

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakePullRequestDiscovery struct {
	prs        []PullRequestRef
	err        error
	subjects   ReviewSubjects
	subjectErr error
}

func (f fakePullRequestDiscovery) PullRequests(context.Context, string) ([]PullRequestRef, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.prs, nil
}

func (f fakePullRequestDiscovery) ViewerReviewSubjects(context.Context) (ReviewSubjects, error) {
	if f.subjectErr != nil {
		return f.subjects, f.subjectErr
	}
	return f.subjects, nil
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

func TestLinkPullRequestsForRepo_AppendsOpenPRBackedItems(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := []WorkItem{
		{ID: "main", Repo: repo, Branch: &BranchRef{Name: "main"}},
	}
	prs := []PullRequestRef{{
		Number:     12,
		Title:      "Review fork work",
		State:      "open",
		URL:        "https://example.test/pr/12",
		HeadOwner:  "contributor",
		HeadBranch: "feature",
		BaseBranch: "main",
	}}

	got := LinkPullRequestsForRepo(repo, items, prs)
	if len(got) != 2 {
		t.Fatalf("expected local item plus PR-backed item, got %+v", got)
	}
	item := got[1]
	if item.ID != "pull-request:0maru/gh-zen:#12" {
		t.Fatalf("expected stable PR-backed ID, got %q", item.ID)
	}
	if item.Repo != repo {
		t.Fatalf("expected repo context to be preserved, got %+v", item.Repo)
	}
	if item.PullRequest == nil || item.PullRequest.Number != 12 {
		t.Fatalf("expected PR metadata, got %+v", item.PullRequest)
	}
	if item.Branch == nil || item.Branch.Name != "feature" || item.Branch.Base != "main" || !item.Branch.RemoteOnly {
		t.Fatalf("expected remote PR branch context, got %+v", item.Branch)
	}
	if item.Local == nil || item.Local.State != LocalMissing || !strings.Contains(item.Local.Summary, "fork head contributor") {
		t.Fatalf("expected missing local fork summary, got %+v", item.Local)
	}
	if item.Checks.State != CheckUnknown {
		t.Fatalf("expected unknown checks placeholder, got %+v", item.Checks)
	}
}

func TestLinkPullRequestsForRepo_DeduplicatesLocalBackedPullRequests(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := []WorkItem{
		{ID: "feature", Repo: repo, Branch: &BranchRef{Name: "feature"}},
	}
	prs := []PullRequestRef{{
		Number:     12,
		State:      "open",
		HeadOwner:  "0maru",
		HeadBranch: "feature",
	}}

	got := LinkPullRequestsForRepo(repo, items, prs)
	if len(got) != 1 {
		t.Fatalf("expected linked PR not to be duplicated, got %+v", got)
	}
	if got[0].PullRequest == nil || got[0].PullRequest.Number != 12 {
		t.Fatalf("expected local item to keep linked PR, got %+v", got[0])
	}
}

func TestLinkPullRequestsForRepo_SkipsClosedPRBackedItems(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := []WorkItem{
		{ID: "main", Repo: repo, Branch: &BranchRef{Name: "main"}},
	}
	prs := []PullRequestRef{{
		Number:     12,
		State:      "closed",
		HeadOwner:  "0maru",
		HeadBranch: "closed-feature",
	}}

	got := LinkPullRequestsForRepo(repo, items, prs)
	if len(got) != 1 {
		t.Fatalf("expected closed unmatched PR to stay hidden, got %+v", got)
	}
}

func TestApplyReviewPerspective_MarksViewerReviewRequestsAndWaitingPRs(t *testing.T) {
	subjects := ReviewSubjects{Login: "0maru", TeamSlugs: []string{"frontend"}}
	prs := []PullRequestRef{
		{
			Number: 1,
			State:  "open",
			ReviewRequests: []ReviewRequestRef{
				{Kind: "User", Login: "0maru"},
			},
		},
		{
			Number:      2,
			State:       "open",
			AuthorLogin: "0maru",
			ReviewRequests: []ReviewRequestRef{
				{Kind: "User", Login: "alice"},
			},
		},
		{
			Number: 3,
			State:  "open",
			ReviewRequests: []ReviewRequestRef{
				{Kind: "Team", Slug: "frontend"},
			},
		},
		{
			Number:  4,
			State:   "open",
			IsDraft: true,
			ReviewRequests: []ReviewRequestRef{
				{Kind: "User", Login: "0maru"},
			},
		},
	}

	got := ApplyReviewPerspective(prs, subjects)
	if !got[0].ViewerReviewRequested {
		t.Fatalf("expected direct viewer review request to be marked: %+v", got[0])
	}
	if !got[1].ViewerAuthored || !got[1].WaitingOnReview {
		t.Fatalf("expected viewer-authored pending request to be marked waiting: %+v", got[1])
	}
	if !got[2].ViewerReviewRequested {
		t.Fatalf("expected viewer team review request to be marked: %+v", got[2])
	}
	if got[3].ViewerReviewRequested {
		t.Fatalf("expected draft PR to be excluded from actionable review requests: %+v", got[3])
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

func TestPullRequestLinkService_ContinuesWhenViewerReviewSubjectDiscoveryFails(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	service := PullRequestLinkService{
		GitHub: fakePullRequestDiscovery{
			prs: []PullRequestRef{{
				Number:     24,
				HeadOwner:  "0maru",
				HeadBranch: "feature",
				State:      "open",
				ReviewRequests: []ReviewRequestRef{
					{Kind: "User", Login: "0maru"},
				},
			}},
			subjects:   ReviewSubjects{Login: "0maru"},
			subjectErr: errors.New("teams failed"),
		},
	}

	got := service.LinkForRepo(context.Background(), repo, []WorkItem{
		{ID: "feature", Repo: repo, Branch: &BranchRef{Name: "feature"}},
	})
	if len(got) != 2 {
		t.Fatalf("expected linked item plus non-fatal error item, got %+v", got)
	}
	if got[0].PullRequest == nil || !got[0].PullRequest.ViewerReviewRequested {
		t.Fatalf("expected partial viewer metadata to mark review request, got %+v", got[0].PullRequest)
	}
	if got[1].Title() != "pull request discovery error" {
		t.Fatalf("expected discovery error item, got %q", got[1].Title())
	}
	if got[1].Local == nil || !strings.Contains(got[1].Local.Summary, "viewer review subject discovery failed") {
		t.Fatalf("expected viewer discovery error summary, got %+v", got[1].Local)
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
