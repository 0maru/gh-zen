package pullrequests

// FakePullRequests returns deterministic PR data for the demo model and tests.
func FakePullRequests() []PullRequest {
	return []PullRequest{
		{
			Number:         24,
			Title:          "Add layered config model",
			State:          "open",
			Author:         "teammate",
			HeadRef:        "0maru/feat/config-loader",
			BaseRef:        "main",
			ReviewDecision: "review requested",
			ReviewRequests: []ReviewRequest{{Kind: "User", Login: "0maru", Name: "0maru"}},
			LatestReviews:  []Review{{Author: "alice", State: "commented"}},
			LinkedIssues:   []LinkedIssue{{Number: 9, Title: "Implement layered config model and merge rules", State: "open", URL: "https://github.com/0maru/gh-zen/issues/9"}},
			Checks:         CheckSummary{State: CheckFailing, Passing: 2, Failing: 1},
			Mergeability:   "mergeable",
			UpdatedAt:      "2026-05-03T12:00:00Z",
			URL:            "https://github.com/0maru/gh-zen/pull/24",
			BodyExcerpt:    "Adds layered configuration defaults, project overrides, and terminal profiles.",
		},
		{
			Number:       31,
			Title:        "Draft repository browser polish",
			State:        "open",
			IsDraft:      true,
			Author:       "0maru",
			HeadRef:      "0maru/feat/browser-polish",
			BaseRef:      "main",
			Checks:       CheckSummary{State: CheckPending, Pending: 2},
			Mergeability: "unknown",
			UpdatedAt:    "2026-05-02T09:30:00Z",
			URL:          "https://github.com/0maru/gh-zen/pull/31",
			BodyExcerpt:  "Polishes the repository browser layout before requesting review.",
		},
		{
			Number:         18,
			Title:          "Fix CI lint warnings",
			State:          "merged",
			Author:         "0maru",
			HeadRef:        "0maru/fix/lint",
			BaseRef:        "main",
			ReviewDecision: "approved",
			LatestReviews:  []Review{{Author: "bob", State: "approved"}},
			Checks:         CheckSummary{State: CheckPassing, Passing: 4},
			Mergeability:   "mergeable",
			UpdatedAt:      "2026-04-30T16:45:00Z",
			URL:            "https://github.com/0maru/gh-zen/pull/18",
			BodyExcerpt:    "Fixes static analysis warnings and keeps CI green.",
		},
	}
}
