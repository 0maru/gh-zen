package workbench

import "time"

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

func FakeWorkflowRuns() []WorkflowRunRef {
	created := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	return []WorkflowRunRef{
		{
			ID:           1001,
			RunNumber:    77,
			WorkflowName: "CI",
			Branch:       "main",
			Event:        "push",
			Status:       "completed",
			Conclusion:   "success",
			Actor:        "0maru",
			HeadSHA:      "3da43e7cafe1234567890abcdef1234567890ab",
			Title:        "Set up GitHub CLI extension foundation",
			URL:          "https://github.com/0maru/gh-zen/actions/runs/1001",
			CreatedAt:    created,
			UpdatedAt:    created.Add(4 * time.Minute),
		},
		{
			ID:           1002,
			RunNumber:    78,
			WorkflowName: "CI",
			Branch:       "feat/config-loader",
			Event:        "pull_request",
			Status:       "completed",
			Conclusion:   "failure",
			Actor:        "teammate",
			HeadSHA:      "a1b2c3d4e5f678901234567890abcdef12345678",
			Title:        "Add config defaults",
			URL:          "https://github.com/0maru/gh-zen/actions/runs/1002",
			CreatedAt:    created.Add(1 * time.Hour),
			UpdatedAt:    created.Add(1*time.Hour + 8*time.Minute),
		},
		{
			ID:           1003,
			RunNumber:    79,
			WorkflowName: "Release",
			Branch:       "main",
			Event:        "workflow_dispatch",
			Status:       "in_progress",
			Actor:        "0maru",
			HeadSHA:      "c3d4e5f678901234567890abcdef1234567890ab",
			Title:        "Build release candidate",
			URL:          "https://github.com/0maru/gh-zen/actions/runs/1003",
			CreatedAt:    created.Add(2 * time.Hour),
			UpdatedAt:    created.Add(2*time.Hour + 2*time.Minute),
		},
	}
}

func FakeWorkflowJobs() map[int64][]WorkflowJobRef {
	started := time.Date(2026, 6, 20, 13, 0, 0, 0, time.UTC)
	return map[int64][]WorkflowJobRef{
		1001: {
			{
				ID:          2001,
				Name:        "test",
				Status:      "completed",
				Conclusion:  "success",
				StartedAt:   started,
				CompletedAt: started.Add(4 * time.Minute),
				URL:         "https://github.com/0maru/gh-zen/actions/runs/1001/job/2001",
				Steps: []WorkflowStepRef{
					{Number: 1, Name: "Checkout", Status: "completed", Conclusion: "success"},
					{Number: 2, Name: "Test", Status: "completed", Conclusion: "success"},
				},
			},
		},
		1002: {
			{
				ID:          2002,
				Name:        "lint",
				Status:      "completed",
				Conclusion:  "success",
				StartedAt:   started.Add(1 * time.Hour),
				CompletedAt: started.Add(1*time.Hour + 2*time.Minute),
				URL:         "https://github.com/0maru/gh-zen/actions/runs/1002/job/2002",
				Steps: []WorkflowStepRef{
					{Number: 1, Name: "Checkout", Status: "completed", Conclusion: "success"},
					{Number: 2, Name: "Lint", Status: "completed", Conclusion: "success"},
				},
			},
			{
				ID:          2003,
				Name:        "test",
				Status:      "completed",
				Conclusion:  "failure",
				StartedAt:   started.Add(1 * time.Hour),
				CompletedAt: started.Add(1*time.Hour + 8*time.Minute),
				URL:         "https://github.com/0maru/gh-zen/actions/runs/1002/job/2003",
				Steps: []WorkflowStepRef{
					{Number: 1, Name: "Checkout", Status: "completed", Conclusion: "success"},
					{Number: 2, Name: "Go test", Status: "completed", Conclusion: "failure"},
				},
			},
		},
		1003: {
			{
				ID:        2004,
				Name:      "build",
				Status:    "in_progress",
				StartedAt: started.Add(2 * time.Hour),
				URL:       "https://github.com/0maru/gh-zen/actions/runs/1003/job/2004",
				Steps: []WorkflowStepRef{
					{Number: 1, Name: "Checkout", Status: "completed", Conclusion: "success"},
					{Number: 2, Name: "Build", Status: "in_progress"},
				},
			},
		},
	}
}

func FakeWorkflowAnnotations() map[int64][]AnnotationRef {
	return map[int64][]AnnotationRef{
		2003: {
			{
				Path:      "internal/app/model.go",
				StartLine: 42,
				EndLine:   42,
				Level:     "failure",
				Title:     "Test failure",
				Message:   "expected Actions preview to include failed job summary",
			},
		},
	}
}

func FakeWorkflowLogs() map[int64]WorkflowLog {
	return map[int64]WorkflowLog{
		1002: {
			RunID:  1002,
			Failed: true,
			Lines: []string{
				"test\tGo test\t--- FAIL: TestActionsPreview",
				"test\tGo test\texpected Actions preview to include failed job summary",
			},
		},
	}
}
