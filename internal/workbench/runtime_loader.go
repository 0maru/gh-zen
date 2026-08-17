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
	Repo                RepoRef
	Repositories        []RepositorySummary
	Items               []WorkItem
	PullRequests        []PullRequestRef
	PullRequestsLoaded  bool
	IssuesRepo          RepoRef
	Issues              []IssueRef
	IssuesLoaded        bool
	IssuesError         string
	LocalDiscoveryError string
	FailedCheckRefs     []string
	ViewerSubject       ReviewSubjects
	ViewerSubjectError  string
}

// RuntimeLoader composes local Git discovery with GitHub workbench enrichment.
type RuntimeLoader struct {
	Repo                      RepoRef
	RepoPath                  string
	Local                     LocalDiscovery
	GitHub                    GitHubWorkbenchDiscovery
	IncludeIssueCommentsCount bool
}

// Load returns workbench items for one repository without failing on partial GitHub discovery errors.
func (l RuntimeLoader) Load(ctx context.Context) RuntimeLoadResult {
	items, localDiscoveryErr := (LocalWorkItemService{
		Repo:      l.Repo,
		RepoPath:  l.RepoPath,
		Discovery: l.Local,
	}).load(ctx)

	result := RuntimeLoadResult{Repo: l.Repo}
	if localDiscoveryErr != nil {
		result.LocalDiscoveryError = localDiscoveryErr.Error()
	}
	if l.GitHub == nil {
		result.Items = items
		return result
	}

	repoName := l.Repo.FullName()
	var pullRequestErrors []error
	subjects, err := reviewSubjects(ctx, l.GitHub)
	if err != nil {
		pullRequestErrors = append(pullRequestErrors, err)
		result.ViewerSubjectError = err.Error()
	}
	result.ViewerSubject = subjects

	prs, err := l.GitHub.PullRequests(ctx, repoName)
	if err != nil {
		pullRequestErrors = append(pullRequestErrors, err)
	} else {
		if !subjects.Empty() {
			prs = ApplyReviewPerspective(prs, subjects)
		}
		result.PullRequests = append([]PullRequestRef(nil), prs...)
		result.PullRequestsLoaded = true
		items = LinkPullRequestsForRepo(l.Repo, items, prs)
	}
	if len(pullRequestErrors) > 0 {
		items = append(cloneWorkItems(items), pullRequestDiscoveryErrorItem(l.Repo, errors.Join(pullRequestErrors...)))
	}

	var discoveryErrors []error
	issues, err := loadIssues(ctx, l.GitHub, repoName, l.IncludeIssueCommentsCount)
	if err != nil {
		discoveryErrors = append(discoveryErrors, err)
		result.IssuesError = err.Error()
	}
	if issues != nil {
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
			result.FailedCheckRefs = append(result.FailedCheckRefs, ref)
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

func loadIssues(ctx context.Context, discovery GitHubWorkbenchDiscovery, repo string, includeComments bool) ([]IssueRef, error) {
	if configurable, ok := discovery.(IssueListDiscovery); ok {
		return configurable.IssuesWithOptions(ctx, repo, IssueListOptions{IncludeCommentsCount: includeComments})
	}
	return discovery.Issues(ctx, repo)
}
