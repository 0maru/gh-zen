package workbench

import (
	"fmt"
	"strconv"
	"strings"
)

// RepositorySummary contains the repository-level preview shown by the TUI.
type RepositorySummary struct {
	Repo                 RepoRef
	Path                 string
	DefaultBranch        string
	Remotes              []string
	ActiveWorktreeCount  int
	OpenPullRequestCount int
	OpenIssueCount       int
	FailingCheckCount    int
}

// RepositoryDiagnostic describes a non-fatal repository discovery warning.
type RepositoryDiagnostic struct {
	Path    string
	Message string
}

// SummarizeRepository derives preview counts from loaded workbench items.
func SummarizeRepository(repo RepoRef, path string, defaultBranch string, remotes []string, items []WorkItem) RepositorySummary {
	summary := RepositorySummary{
		Repo:          repo,
		Path:          path,
		DefaultBranch: defaultBranch,
		Remotes:       append([]string(nil), remotes...),
	}
	openPullRequests := map[string]struct{}{}
	openIssues := map[string]struct{}{}
	for _, item := range items {
		if item.Repo != repo {
			continue
		}
		if item.Worktree != nil && item.Local != nil && item.Local.State != LocalMissing {
			summary.ActiveWorktreeCount++
		}
		if item.PullRequest != nil && strings.EqualFold(item.PullRequest.State, "open") {
			openPullRequests[pullRequestSummaryKey(*item.PullRequest)] = struct{}{}
		}
		if item.Issue != nil && strings.EqualFold(item.Issue.State, "open") {
			openIssues[issueSummaryKey(*item.Issue)] = struct{}{}
		}
		if item.Checks.State == CheckFailing {
			if item.Checks.Failing > 0 {
				summary.FailingCheckCount += item.Checks.Failing
			} else {
				summary.FailingCheckCount++
			}
		}
	}
	summary.OpenPullRequestCount = len(openPullRequests)
	summary.OpenIssueCount = len(openIssues)
	return summary
}

// RepositoryPathErrorItem exposes path and root diagnostics without hiding
// repository work that loaded successfully.
func RepositoryPathErrorItem(repo RepoRef, diagnostics []RepositoryDiagnostic) WorkItem {
	summary := "repository path resolution failed"
	if len(diagnostics) > 0 {
		summary += ": " + DiagnosticSummary(diagnostics)
	}
	return WorkItem{
		ID:     "repository-path-error:" + repo.FullName() + ":" + repositoryDiagnosticID(diagnostics),
		Repo:   repo,
		Branch: &BranchRef{Name: "repository discovery error"},
		Local: &LocalStatus{
			State:   LocalUnknown,
			Summary: summary,
		},
		Checks: CheckSummary{State: CheckUnknown},
	}
}

// DiagnosticSummary formats discovery diagnostics for compact UI display.
func DiagnosticSummary(diagnostics []RepositoryDiagnostic) string {
	parts := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if diagnostic.Path == "" {
			parts = append(parts, diagnostic.Message)
			continue
		}
		parts = append(parts, diagnostic.Path+": "+diagnostic.Message)
	}
	return strings.Join(parts, "; ")
}

func pullRequestSummaryKey(pr PullRequestRef) string {
	if pr.Number > 0 {
		return strconv.Itoa(pr.Number)
	}
	return pr.HeadOwner + ":" + pr.HeadBranch
}

func issueSummaryKey(issue IssueRef) string {
	if issue.Number > 0 {
		return strconv.Itoa(issue.Number)
	}
	return issue.Title
}

func repositoryDiagnosticID(diagnostics []RepositoryDiagnostic) string {
	if len(diagnostics) == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d", len(diagnostics))
}
