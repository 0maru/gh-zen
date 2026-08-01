package workbench

import (
	"context"
	"errors"
)

// GitHubWorkbenchDiscovery provides GitHub data needed by the runtime workbench loader.
type GitHubWorkbenchDiscovery interface {
	PullRequestDiscovery
	IssueCheckDiscovery
}

// RuntimeLoadResult contains refreshed workbench data.
type RuntimeLoadResult struct {
	Repo               RepoRef
	Repositories       []RepositorySummary
	Items              []WorkItem
	PullRequests       []PullRequestRef
	PullRequestsLoaded bool
	IssuesRepo         RepoRef
	Issues             []IssueRef
	IssuesLoaded       bool
	ViewerSubject      ReviewSubjects
}

// RuntimeLoader composes local Git discovery with GitHub workbench enrichment.
type RuntimeLoader struct {
	Repo     RepoRef
	RepoPath string
	Local    LocalDiscovery
	GitHub   GitHubWorkbenchDiscovery
}

// Load returns workbench items for one repository without failing on partial GitHub discovery errors.
func (l RuntimeLoader) Load(ctx context.Context) RuntimeLoadResult {
	items := (LocalWorkItemService{
		Repo:      l.Repo,
		RepoPath:  l.RepoPath,
		Discovery: l.Local,
	}).WorkItems(ctx)

	result := RuntimeLoadResult{Repo: l.Repo}
	if l.GitHub == nil {
		result.Items = items
		return result
	}

	repoName := l.Repo.FullName()
	prs, err := l.GitHub.PullRequests(ctx, repoName)
	if err != nil {
		items = append(cloneWorkItems(items), pullRequestDiscoveryErrorItem(l.Repo, err))
	} else {
		var discoveryErrors []error
		subjects, err := reviewSubjects(ctx, l.GitHub)
		if err != nil {
			discoveryErrors = append(discoveryErrors, err)
		}
		if !subjects.Empty() {
			prs = ApplyReviewPerspective(prs, subjects)
		}
		result.PullRequests = append([]PullRequestRef(nil), prs...)
		result.PullRequestsLoaded = true
		result.ViewerSubject = subjects
		items = LinkPullRequestsForRepo(l.Repo, items, prs)
		if len(discoveryErrors) > 0 {
			items = append(items, pullRequestDiscoveryErrorItem(l.Repo, errors.Join(discoveryErrors...)))
		}
	}

	var discoveryErrors []error
	issues, err := l.GitHub.Issues(ctx, repoName)
	if err != nil {
		discoveryErrors = append(discoveryErrors, err)
	} else {
		result.IssuesRepo = l.Repo
		result.Issues = append([]IssueRef(nil), issues...)
		result.IssuesLoaded = true
	}

	items = LinkIssues(items, issues)
	for i := range items {
		if items[i].PullRequest == nil {
			continue
		}
		ref := pullRequestCheckRef(items[i])
		if ref == "" {
			continue
		}
		checks, err := l.GitHub.CheckSummary(ctx, repoName, ref)
		if err != nil {
			discoveryErrors = append(discoveryErrors, err)
			if items[i].Checks.State == "" {
				items[i].Checks = CheckSummary{State: CheckUnknown}
			}
			continue
		}
		if checks.State != "" {
			items[i].Checks = checks
		}
	}
	if len(discoveryErrors) > 0 {
		items = append(items, issueCheckDiscoveryErrorItem(l.Repo, errors.Join(discoveryErrors...)))
	}

	result.Items = items
	return result
}
