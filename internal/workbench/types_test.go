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
