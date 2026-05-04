package github

import (
	"context"

	"github.com/0maru/gh-zen/internal/workbench"
)

// FakeService is an in-memory GitHub service for small tests.
type FakeService struct {
	Summaries          map[string]RepositorySummary
	PullRequestsByRepo map[string][]workbench.PullRequestRef
	IssuesByRepo       map[string][]workbench.IssueRef
	Checks             map[string]workbench.CheckSummary
	ReviewSubjects     workbench.ReviewSubjects
	Err                error
}

func (f FakeService) RepositorySummary(_ context.Context, repo string) (RepositorySummary, error) {
	if f.Err != nil {
		return RepositorySummary{}, f.Err
	}
	if summary, ok := f.Summaries[repo]; ok {
		return summary, nil
	}
	prs, err := f.PullRequestsForRepo(repo)
	if err != nil {
		return RepositorySummary{}, err
	}
	issues, err := f.IssuesForRepo(repo)
	if err != nil {
		return RepositorySummary{}, err
	}
	return RepositorySummary{
		Repo:         repo,
		PullRequests: prs,
		Issues:       issues,
		Checks:       fakeCheckSummary(f.Checks, repo),
	}, nil
}

func (f FakeService) PullRequests(_ context.Context, repo string) ([]workbench.PullRequestRef, error) {
	return f.PullRequestsForRepo(repo)
}

func (f FakeService) Issues(_ context.Context, repo string) ([]workbench.IssueRef, error) {
	return f.IssuesForRepo(repo)
}

func (f FakeService) CheckSummary(_ context.Context, repo string, ref string) (workbench.CheckSummary, error) {
	if f.Err != nil {
		return workbench.CheckSummary{}, f.Err
	}
	if summary, ok := f.Checks[repo+"@"+ref]; ok {
		return summary, nil
	}
	if summary, ok := f.Checks[repo]; ok {
		return summary, nil
	}
	return workbench.CheckSummary{State: workbench.CheckUnknown}, nil
}

func (f FakeService) ViewerReviewSubjects(context.Context) (workbench.ReviewSubjects, error) {
	if f.Err != nil {
		return workbench.ReviewSubjects{}, f.Err
	}
	return f.ReviewSubjects, nil
}

func (f FakeService) PullRequestsForRepo(repo string) ([]workbench.PullRequestRef, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return append([]workbench.PullRequestRef(nil), f.PullRequestsByRepo[repo]...), nil
}

func (f FakeService) IssuesForRepo(repo string) ([]workbench.IssueRef, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return append([]workbench.IssueRef(nil), f.IssuesByRepo[repo]...), nil
}

func fakeCheckSummary(checks map[string]workbench.CheckSummary, key string) workbench.CheckSummary {
	if summary, ok := checks[key]; ok {
		return summary
	}
	return workbench.CheckSummary{State: workbench.CheckUnknown}
}
