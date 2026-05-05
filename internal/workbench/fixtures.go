package workbench

func FakeRepos() []RepoRef {
	return []RepoRef{
		{Owner: "0maru", Name: "gh-zen"},
		{Owner: "0maru", Name: "dotfiles"},
	}
}

func FakeWorkItems() []WorkItem {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	return []WorkItem{
		{
			ID:       "worktree-main",
			Repo:     repo,
			Branch:   &BranchRef{Name: "main", Base: "origin/main"},
			Worktree: &WorktreeRef{Path: "~/workspaces/github.com/0maru/gh-zen"},
			Checks:   CheckSummary{State: CheckPassing, Passing: 3},
			Local:    &LocalStatus{State: LocalClean},
			Commits: []CommitRef{
				{ShortSHA: "3da43e7", Subject: "Set up GitHub CLI extension foundation"},
			},
		},
		{
			ID:       "worktree-config-loader",
			Repo:     repo,
			Branch:   &BranchRef{Name: "feat/config-loader", Base: "main"},
			Worktree: &WorktreeRef{Path: "~/workspaces/github.com/0maru/gh-zen-agent-a"},
			Issue:    &IssueRef{Number: 9, Title: "Implement layered config model and merge rules", State: "open", URL: "https://github.com/0maru/gh-zen/issues/9", Certain: true},
			PullRequest: &PullRequestRef{
				Number:                24,
				Title:                 "Add layered config model",
				State:                 "open",
				URL:                   "https://github.com/0maru/gh-zen/pull/24",
				AuthorLogin:           "teammate",
				HeadOwner:             "0maru",
				HeadBranch:            "feat/config-loader",
				BaseBranch:            "main",
				ReviewState:           "review requested",
				ReviewRequests:        []ReviewRequestRef{{Kind: "User", Login: "0maru", Name: "0maru"}},
				ViewerReviewRequested: true,
			},
			Checks: CheckSummary{State: CheckFailing, Passing: 2, Failing: 1},
			Local:  &LocalStatus{State: LocalDirty, Summary: "3 files changed"},
			Commits: []CommitRef{
				{ShortSHA: "a1b2c3d", Subject: "Add config defaults"},
				{ShortSHA: "b2c3d4e", Subject: "Test merge precedence"},
			},
		},
		{
			ID:       "worktree-repo-workbench",
			Repo:     repo,
			Branch:   &BranchRef{Name: "feat/repo-workbench", Base: "main"},
			Worktree: &WorktreeRef{Path: "~/workspaces/github.com/0maru/gh-zen-agent-b"},
			Issue:    &IssueRef{Number: 6, Title: "Build fake Repository Workbench shell", State: "open", URL: "https://github.com/0maru/gh-zen/issues/6", Certain: true},
			Checks:   CheckSummary{State: CheckPending, Pending: 2},
			Local:    &LocalStatus{State: LocalClean},
			Commits: []CommitRef{
				{ShortSHA: "c3d4e5f", Subject: "Render work item list"},
			},
		},
		{
			ID:     "remote-preview-state",
			Repo:   repo,
			Branch: &BranchRef{Name: "agent/preview-state", Base: "main", RemoteOnly: true},
			Issue:  &IssueRef{Number: 8, Title: "Add asynchronous preview state machine", State: "open", URL: "https://github.com/0maru/gh-zen/issues/8", Certain: true},
			Checks: CheckSummary{State: CheckUnknown},
			Local:  &LocalStatus{State: LocalMissing, Summary: "no local worktree"},
		},
		{
			ID:     "issue-branch-preview-ux",
			Repo:   repo,
			Issue:  &IssueRef{Number: 34, Title: "Branch preview UX", State: "open", URL: "https://github.com/0maru/gh-zen/issues/34", Certain: false, Source: IssueLinkSourceBranch},
			Checks: CheckSummary{State: CheckUnknown},
			Local:  &LocalStatus{State: LocalUnknown, Summary: "unstarted"},
		},
	}
}
