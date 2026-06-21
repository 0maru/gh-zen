package pullrequests

import (
	"fmt"
	"strings"
)

// PullRequestFilter configures the PR browser list.
type PullRequestFilter struct {
	State           StateFilter
	Author          string
	ReviewRequested bool
	WaitingOnReview bool
	FailedChecks    bool
	Draft           DraftFilter
	TextQuery       string
}

// Filter returns a sorted copy of pull requests that match the filter.
func Filter(prs []PullRequest, filter PullRequestFilter) []PullRequest {
	normalized := filter.Normalize()
	out := make([]PullRequest, 0, len(prs))
	for _, pr := range prs {
		if normalized.Matches(pr) {
			out = append(out, pr)
		}
	}
	SortByUpdatedDesc(out)
	return out
}

func (f PullRequestFilter) Normalize() PullRequestFilter {
	f.State = normalizeStateFilter(f.State)
	f.Draft = normalizeDraftFilter(f.Draft)
	f.Author = strings.TrimSpace(f.Author)
	f.TextQuery = strings.TrimSpace(f.TextQuery)
	return f
}

func (f PullRequestFilter) Matches(pr PullRequest) bool {
	f = f.Normalize()
	if f.State != StateAny && !strings.EqualFold(pr.State, string(f.State)) {
		return false
	}
	switch f.Draft {
	case DraftOnly:
		if !pr.IsDraft {
			return false
		}
	case DraftNonDraft:
		if pr.IsDraft {
			return false
		}
	}
	if f.Author != "" && !strings.EqualFold(f.Author, pr.Author) {
		return false
	}
	if f.ReviewRequested && len(pr.ReviewRequests) == 0 && !pr.ViewerReviewRequested {
		return false
	}
	if f.WaitingOnReview && !pr.WaitingOnReview && !isWaitingOnReview(pr) {
		return false
	}
	if f.FailedChecks && pr.Checks.State != CheckFailing {
		return false
	}
	if f.TextQuery != "" && !matchesTextQuery(pr, f.TextQuery) {
		return false
	}
	return true
}

func (f PullRequestFilter) ActiveLabels() []string {
	f = f.Normalize()
	labels := []string{}
	if f.State != StateAny {
		labels = append(labels, "state:"+string(f.State))
	}
	if f.Author != "" {
		labels = append(labels, "author:"+f.Author)
	}
	if f.ReviewRequested {
		labels = append(labels, "review requested")
	}
	if f.WaitingOnReview {
		labels = append(labels, "waiting on review")
	}
	if f.FailedChecks {
		labels = append(labels, "failed checks")
	}
	switch f.Draft {
	case DraftOnly:
		labels = append(labels, "draft")
	case DraftNonDraft:
		labels = append(labels, "non-draft")
	}
	if f.TextQuery != "" {
		labels = append(labels, "search:"+f.TextQuery)
	}
	return labels
}

func (f PullRequestFilter) Active() bool {
	return len(f.ActiveLabels()) > 0
}

func (f PullRequestFilter) NextState() PullRequestFilter {
	switch f.Normalize().State {
	case StateAny:
		f.State = StateOpen
	case StateOpen:
		f.State = StateClosed
	case StateClosed:
		f.State = StateMerged
	default:
		f.State = StateAny
	}
	return f
}

func (f PullRequestFilter) NextDraft() PullRequestFilter {
	switch f.Normalize().Draft {
	case DraftAny:
		f.Draft = DraftOnly
	case DraftOnly:
		f.Draft = DraftNonDraft
	default:
		f.Draft = DraftAny
	}
	return f
}

func normalizeStateFilter(value StateFilter) StateFilter {
	switch StateFilter(strings.ToLower(strings.TrimSpace(string(value)))) {
	case StateOpen:
		return StateOpen
	case StateClosed:
		return StateClosed
	case StateMerged:
		return StateMerged
	default:
		return StateAny
	}
}

func normalizeDraftFilter(value DraftFilter) DraftFilter {
	switch DraftFilter(strings.ToLower(strings.TrimSpace(string(value)))) {
	case DraftOnly:
		return DraftOnly
	case DraftNonDraft:
		return DraftNonDraft
	default:
		return DraftAny
	}
}

func isWaitingOnReview(pr PullRequest) bool {
	if pr.IsDraft || !strings.EqualFold(pr.State, "open") {
		return false
	}
	decision := strings.ToLower(pr.ReviewDecision)
	return len(pr.ReviewRequests) > 0 || strings.Contains(decision, "review required")
}

func matchesTextQuery(pr PullRequest, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	fields := []string{
		pr.Title,
		pr.Author,
		pr.HeadRef,
		pr.BaseRef,
		pr.ReviewDecision,
		pr.Mergeability,
		pr.BodyExcerpt,
		fmt.Sprintf("#%d", pr.Number),
		fmt.Sprintf("%d", pr.Number),
	}
	for _, issue := range pr.LinkedIssues {
		fields = append(fields, issue.Label(), issue.State)
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}
