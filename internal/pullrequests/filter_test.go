package pullrequests

import (
	"reflect"
	"testing"
)

func TestFilter_Combinatorics(t *testing.T) {
	prs := []PullRequest{
		{
			Number:          1,
			Title:           "Open ready",
			State:           "open",
			Author:          "alice",
			HeadRef:         "alice/feature",
			BaseRef:         "main",
			ReviewDecision:  "review required",
			ReviewRequests:  []ReviewRequest{{Login: "bob"}},
			WaitingOnReview: true,
			Checks:          CheckSummary{State: CheckFailing, Failing: 1},
			UpdatedAt:       "2026-05-03T12:00:00Z",
			BodyExcerpt:     "Closes #10",
			LinkedIssues:    []LinkedIssue{{Number: 10, Title: "Searchable issue"}},
		},
		{
			Number:    2,
			Title:     "Draft work",
			State:     "open",
			IsDraft:   true,
			Author:    "alice",
			HeadRef:   "alice/draft",
			BaseRef:   "main",
			Checks:    CheckSummary{State: CheckPending, Pending: 1},
			UpdatedAt: "2026-05-04T12:00:00Z",
		},
		{
			Number:    3,
			Title:     "Merged work",
			State:     "merged",
			Author:    "carol",
			HeadRef:   "carol/merged",
			BaseRef:   "main",
			Checks:    CheckSummary{State: CheckPassing, Passing: 2},
			UpdatedAt: "2026-05-01T12:00:00Z",
		},
		{
			Number:    4,
			Title:     "Closed work",
			State:     "closed",
			Author:    "dave",
			HeadRef:   "dave/closed",
			BaseRef:   "main",
			Checks:    CheckSummary{State: CheckUnknown},
			UpdatedAt: "2026-05-02T12:00:00Z",
		},
	}

	cases := []struct {
		name   string
		filter PullRequestFilter
		want   []int
	}{
		{name: "default sorts updated desc", filter: PullRequestFilter{}, want: []int{2, 1, 4, 3}},
		{name: "state open", filter: PullRequestFilter{State: StateOpen}, want: []int{2, 1}},
		{name: "state closed", filter: PullRequestFilter{State: StateClosed}, want: []int{4}},
		{name: "state merged", filter: PullRequestFilter{State: StateMerged}, want: []int{3}},
		{name: "author", filter: PullRequestFilter{Author: "ALICE"}, want: []int{2, 1}},
		{name: "review requested", filter: PullRequestFilter{ReviewRequested: true}, want: []int{1}},
		{name: "waiting on review", filter: PullRequestFilter{WaitingOnReview: true}, want: []int{1}},
		{name: "failed checks", filter: PullRequestFilter{FailedChecks: true}, want: []int{1}},
		{name: "draft only", filter: PullRequestFilter{Draft: DraftOnly}, want: []int{2}},
		{name: "non draft", filter: PullRequestFilter{Draft: DraftNonDraft}, want: []int{1, 4, 3}},
		{name: "text title", filter: PullRequestFilter{TextQuery: "merged"}, want: []int{3}},
		{name: "text issue", filter: PullRequestFilter{TextQuery: "#10"}, want: []int{1}},
		{name: "combined", filter: PullRequestFilter{State: StateOpen, Author: "alice", Draft: DraftNonDraft, FailedChecks: true}, want: []int{1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := prNumbers(Filter(prs, tc.filter))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected numbers %+v, got %+v", tc.want, got)
			}
		})
	}
}

func TestFilter_WaitingOnReviewRequiresExplicitViewerScopedFlag(t *testing.T) {
	prs := []PullRequest{
		{
			Number:         1,
			Title:          "Needs my review",
			State:          "open",
			ReviewDecision: "review required",
			ReviewRequests: []ReviewRequest{{Login: "0maru"}},
		},
	}

	got := Filter(prs, PullRequestFilter{WaitingOnReview: true})
	if len(got) != 0 {
		t.Fatalf("expected review-requested PR not to match waiting-on-review filter, got %+v", got)
	}
}

func TestFilter_ActiveLabels(t *testing.T) {
	got := PullRequestFilter{
		State:           StateOpen,
		Author:          "alice",
		ReviewRequested: true,
		WaitingOnReview: true,
		FailedChecks:    true,
		Draft:           DraftOnly,
		TextQuery:       "config",
	}.ActiveLabels()
	want := []string{"state:open", "author:alice", "review requested", "waiting on review", "failed checks", "draft", "search:config"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected labels %+v, got %+v", want, got)
	}
}

func prNumbers(prs []PullRequest) []int {
	numbers := make([]int, 0, len(prs))
	for _, pr := range prs {
		numbers = append(numbers, pr.Number)
	}
	return numbers
}
