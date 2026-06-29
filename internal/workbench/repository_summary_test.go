package workbench

import (
	"strings"
	"testing"
)

func TestSummarizeRepositoryCountsPreviewFields(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	other := RepoRef{Owner: "0maru", Name: "dotfiles"}
	items := []WorkItem{
		{
			ID:          "worktree:feature",
			Repo:        repo,
			Worktree:    &WorktreeRef{Path: "/repo-feature"},
			PullRequest: &PullRequestRef{Number: 24, State: "open"},
			Issue:       &IssueRef{Number: 74, State: "open"},
			Checks:      CheckSummary{State: CheckFailing, Failing: 2},
			Local:       &LocalStatus{State: LocalDirty},
		},
		{
			ID:          "pull-request:duplicate",
			Repo:        repo,
			PullRequest: &PullRequestRef{Number: 24, State: "open"},
			Issue:       &IssueRef{Number: 74, State: "open"},
			Checks:      CheckSummary{State: CheckPassing, Passing: 1},
			Local:       &LocalStatus{State: LocalMissing},
		},
		{
			ID:          "pull-request:closed",
			Repo:        repo,
			PullRequest: &PullRequestRef{Number: 25, State: "closed"},
			Issue:       &IssueRef{Number: 75, State: "closed"},
			Checks:      CheckSummary{State: CheckFailing},
			Local:       &LocalStatus{State: LocalMissing},
		},
		{
			ID:          "worktree:other",
			Repo:        other,
			Worktree:    &WorktreeRef{Path: "/dotfiles"},
			PullRequest: &PullRequestRef{Number: 1, State: "open"},
			Issue:       &IssueRef{Number: 1, State: "open"},
			Local:       &LocalStatus{State: LocalClean},
		},
	}

	got := SummarizeRepository(repo, "/repo", "main", []string{"origin"}, items)

	if got.Repo != repo || got.Path != "/repo" || got.DefaultBranch != "main" {
		t.Fatalf("expected repository identity fields, got %+v", got)
	}
	if len(got.Remotes) != 1 || got.Remotes[0] != "origin" {
		t.Fatalf("expected remotes to be copied, got %+v", got.Remotes)
	}
	if got.ActiveWorktreeCount != 1 {
		t.Fatalf("expected one active worktree, got %d", got.ActiveWorktreeCount)
	}
	if got.OpenPullRequestCount != 1 || got.OpenIssueCount != 1 {
		t.Fatalf("expected unique open PR/issue counts, got prs=%d issues=%d", got.OpenPullRequestCount, got.OpenIssueCount)
	}
	if got.FailingCheckCount != 3 {
		t.Fatalf("expected failing check count to include explicit and implicit failures, got %d", got.FailingCheckCount)
	}
}

func TestRepositoryPathErrorItemFormatsDiagnostics(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	item := RepositoryPathErrorItem(repo, []RepositoryDiagnostic{{
		Path:    "repos.roots[0]",
		Message: "\"/missing\" is not accessible",
	}})

	if !strings.HasPrefix(item.ID, "repository-path-error:0maru/gh-zen:") {
		t.Fatalf("expected repository path error ID, got %q", item.ID)
	}
	if item.Title() != "repository discovery error" {
		t.Fatalf("expected repository discovery error title, got %q", item.Title())
	}
	if item.Local == nil || !strings.Contains(item.Local.Summary, "repos.roots[0]") || !strings.Contains(item.Local.Summary, "not accessible") {
		t.Fatalf("expected diagnostic summary, got %+v", item.Local)
	}
}
