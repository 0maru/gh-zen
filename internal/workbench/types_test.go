package workbench

import "testing"

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
