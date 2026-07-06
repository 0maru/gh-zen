package workbench

import (
	"strings"
	"testing"
	"time"
)

func TestRepoRef_FullName_ZeroValueSafe(t *testing.T) {
	if got := (RepoRef{}).FullName(); got != "unknown repo" {
		t.Fatalf("expected unknown repo, got %q", got)
	}
}

func TestWorkItem_ZeroValueSafeLabels(t *testing.T) {
	item := WorkItem{}
	if got := item.Title(); got != "untracked work" {
		t.Fatalf("expected zero-value title to be safe, got %q", got)
	}
	if got := item.Location(); got != "unknown repo" {
		t.Fatalf("expected zero-value location to be safe, got %q", got)
	}
	if got := item.LocalLabel(); got != "unknown" {
		t.Fatalf("expected zero-value local label to be safe, got %q", got)
	}
	if got := item.PullRequestLabel(); got != "no PR" {
		t.Fatalf("expected zero-value PR label to be safe, got %q", got)
	}
	if got := item.IssueLabel(); got != "no issue" {
		t.Fatalf("expected zero-value issue label to be safe, got %q", got)
	}
}

func TestWorkItem_PullRequestLabel_PartialDataSafe(t *testing.T) {
	cases := []struct {
		name string
		pr   *PullRequestRef
		want string
	}{
		{name: "missing state and number", pr: &PullRequestRef{}, want: "PR"},
		{name: "number only", pr: &PullRequestRef{Number: 24}, want: "PR #24"},
		{name: "state", pr: &PullRequestRef{Number: 24, State: "open"}, want: "PR #24 open"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := WorkItem{PullRequest: tc.pr}
			if got := item.PullRequestLabel(); got != tc.want {
				t.Fatalf("expected PR label %q, got %q", tc.want, got)
			}
		})
	}
}

func TestWorkItem_IssueLabel_MarksUncertainIssue(t *testing.T) {
	item := WorkItem{Issue: &IssueRef{Number: 34, Title: "Branch preview UX"}}
	if got := item.IssueLabel(); got != "#34 Branch preview UX (uncertain)" {
		t.Fatalf("expected uncertain issue label, got %q", got)
	}
}

func TestPullRequestRef_HeadLabel(t *testing.T) {
	pr := PullRequestRef{HeadOwner: "contributor", HeadBranch: "feature"}
	if got := pr.HeadLabel(); got != "contributor/feature" {
		t.Fatalf("expected owner-qualified head label, got %q", got)
	}
}

func TestWorkflowRunRef_HoldsGitHubActionsFields(t *testing.T) {
	created := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	updated := created.Add(4 * time.Minute)
	run := WorkflowRunRef{
		ID:           1001,
		RunNumber:    77,
		WorkflowName: "CI",
		Branch:       "main",
		Event:        "push",
		Status:       "completed",
		Conclusion:   "success",
		Actor:        "0maru",
		HeadSHA:      "abcdef1234567890",
		Title:        "Build main",
		URL:          "https://example.test/runs/1001",
		CreatedAt:    created,
		UpdatedAt:    updated,
	}

	if got := run.Label(); got != "run #77 Build main" {
		t.Fatalf("expected run label, got %q", got)
	}
	if got := run.NumberLabel(); got != "run #77" {
		t.Fatalf("expected run number label, got %q", got)
	}
	if got := run.StatusLabel(); got != "success" {
		t.Fatalf("expected conclusion to win status label, got %q", got)
	}
	if got := run.ShortSHA(); got != "abcdef1" {
		t.Fatalf("expected short SHA, got %q", got)
	}
	if run.CreatedAt != created || run.UpdatedAt != updated {
		t.Fatalf("expected timestamps to round-trip, got %+v", run)
	}
}

func TestWorkflowJobAndAnnotationLabels(t *testing.T) {
	job := WorkflowJobRef{ID: 2001, Name: "test", Status: "completed", Conclusion: "failure"}
	if got := job.Label(); got != "test" {
		t.Fatalf("expected job label, got %q", got)
	}
	if got := job.StatusLabel(); got != "failure" {
		t.Fatalf("expected job status label, got %q", got)
	}

	annotation := AnnotationRef{Path: "internal/app/model.go", StartLine: 42, Title: "Test failure"}
	if got := annotation.Label(); got != "internal/app/model.go:42 Test failure" {
		t.Fatalf("expected annotation label, got %q", got)
	}
}

func TestWorkItem_LocationShowsPullRequestHeadForRemotePRBranch(t *testing.T) {
	item := WorkItem{
		Repo:        RepoRef{Owner: "0maru", Name: "gh-zen"},
		Branch:      &BranchRef{Name: "feature", RemoteOnly: true},
		PullRequest: &PullRequestRef{HeadOwner: "contributor", HeadBranch: "feature"},
	}
	if got := item.Location(); got != "contributor/feature" {
		t.Fatalf("expected PR head location, got %q", got)
	}
}

func TestFakeWorkflowRuns_CoverRequiredShapes(t *testing.T) {
	runs := FakeWorkflowRuns()
	if len(runs) < 3 {
		t.Fatalf("expected at least three fake workflow runs, got %d", len(runs))
	}

	var hasSuccess bool
	var hasFailure bool
	var hasInProgress bool
	for _, run := range runs {
		if run.ID == 0 || run.RunNumber == 0 || run.WorkflowName == "" || run.Branch == "" || run.Event == "" || run.Status == "" || run.Actor == "" || run.HeadSHA == "" || run.Title == "" || run.URL == "" || run.CreatedAt.IsZero() || run.UpdatedAt.IsZero() {
			t.Fatalf("fake workflow run missing required field: %+v", run)
		}
		switch {
		case strings.EqualFold(run.Conclusion, "success"):
			hasSuccess = true
		case strings.EqualFold(run.Conclusion, "failure"):
			hasFailure = true
		case strings.EqualFold(run.Status, "in_progress"):
			hasInProgress = true
		}
	}
	if !hasSuccess || !hasFailure || !hasInProgress {
		t.Fatalf("expected success, failure, and in-progress fake runs, got %+v", runs)
	}
	if len(FakeWorkflowJobs()[runs[1].ID]) == 0 {
		t.Fatalf("expected fake jobs for failing run")
	}
	if len(FakeWorkflowAnnotations()) == 0 {
		t.Fatalf("expected fake annotations")
	}
	if len(FakeWorkflowLogs()[runs[1].ID].Lines) == 0 {
		t.Fatalf("expected fake failed logs")
	}
}

func TestFakeWorkItems_CoverRequiredShapes(t *testing.T) {
	items := FakeWorkItems()
	if len(items) < 5 {
		t.Fatalf("expected at least five fake work items, got %d", len(items))
	}

	var hasCleanWorktree bool
	var hasDirtyWorktree bool
	var hasPullRequest bool
	var hasRemoteOnly bool
	var hasIssueOnly bool

	seenIDs := map[string]bool{}
	for _, item := range items {
		if item.ID == "" {
			t.Fatalf("fake work item has empty ID: %+v", item)
		}
		if seenIDs[item.ID] {
			t.Fatalf("duplicate fake work item ID %q", item.ID)
		}
		seenIDs[item.ID] = true

		if item.Worktree != nil && item.Local != nil && item.Local.State == LocalClean {
			hasCleanWorktree = true
		}
		if item.Worktree != nil && item.Local != nil && item.Local.State == LocalDirty {
			hasDirtyWorktree = true
		}
		if item.PullRequest != nil {
			hasPullRequest = true
		}
		if item.Worktree == nil && item.Branch != nil && item.Branch.RemoteOnly {
			hasRemoteOnly = true
		}
		if item.Worktree == nil && item.Branch == nil && item.PullRequest == nil && item.Issue != nil {
			hasIssueOnly = true
		}
	}

	cases := []struct {
		name string
		got  bool
	}{
		{"clean worktree", hasCleanWorktree},
		{"dirty worktree", hasDirtyWorktree},
		{"PR-linked work", hasPullRequest},
		{"remote-only branch", hasRemoteOnly},
		{"issue-only work", hasIssueOnly},
	}
	for _, tc := range cases {
		if !tc.got {
			t.Fatalf("fake work items are missing %s coverage", tc.name)
		}
	}
}
