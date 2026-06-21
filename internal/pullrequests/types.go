package pullrequests

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	StateAny    StateFilter = "any"
	StateOpen   StateFilter = "open"
	StateClosed StateFilter = "closed"
	StateMerged StateFilter = "merged"

	DraftAny      DraftFilter = "any"
	DraftOnly     DraftFilter = "draft"
	DraftNonDraft DraftFilter = "non_draft"

	CheckUnknown CheckState = "unknown"
	CheckPassing CheckState = "passing"
	CheckFailing CheckState = "failing"
	CheckPending CheckState = "pending"
)

// StateFilter selects one GitHub pull request lifecycle state.
type StateFilter string

// DraftFilter controls draft pull request visibility.
type DraftFilter string

// CheckState is a compact rollup of pull request check state.
type CheckState string

// Service loads pull request data for the selected repository.
type Service interface {
	List(ctx context.Context, repo string, filter PullRequestFilter) ([]PullRequest, error)
	Detail(ctx context.Context, repo string, number int) (PullRequest, error)
}

// PullRequest contains the data needed by the first-class PR browser.
type PullRequest struct {
	Number                int
	Title                 string
	State                 string
	IsDraft               bool
	Author                string
	HeadRef               string
	BaseRef               string
	ReviewDecision        string
	ReviewRequests        []ReviewRequest
	LatestReviews         []Review
	LinkedIssues          []LinkedIssue
	Checks                CheckSummary
	Mergeability          string
	UpdatedAt             string
	URL                   string
	BodyExcerpt           string
	ViewerReviewRequested bool
	WaitingOnReview       bool
}

func (p PullRequest) Key() string {
	if p.Number == 0 {
		return "pr:unknown"
	}
	return fmt.Sprintf("pr:%d", p.Number)
}

func (p PullRequest) NumberLabel() string {
	if p.Number == 0 {
		return "PR"
	}
	return fmt.Sprintf("PR #%d", p.Number)
}

func (p PullRequest) ShortNumberLabel() string {
	if p.Number == 0 {
		return "#?"
	}
	return fmt.Sprintf("#%d", p.Number)
}

func (p PullRequest) StateLabel() string {
	state := strings.ToLower(strings.TrimSpace(p.State))
	if state == "" {
		state = "unknown"
	}
	if p.IsDraft {
		return state + " draft"
	}
	return state
}

func (p PullRequest) HeadLabel() string {
	return strings.TrimSpace(p.HeadRef)
}

// ReviewRequest is one requested reviewer or team.
type ReviewRequest struct {
	Kind  string
	Login string
	Name  string
	Slug  string
}

// Review is one latest review entry.
type Review struct {
	Author string
	State  string
}

// LinkedIssue is an issue linked by closing references or PR body text.
type LinkedIssue struct {
	Number int
	Title  string
	State  string
	URL    string
}

func (i LinkedIssue) Label() string {
	if i.Number == 0 {
		return "issue"
	}
	if i.Title == "" {
		return fmt.Sprintf("#%d", i.Number)
	}
	return fmt.Sprintf("#%d %s", i.Number, i.Title)
}

// CheckSummary is a PR-level check rollup.
type CheckSummary struct {
	State   CheckState
	Passing int
	Failing int
	Pending int
}

func (c CheckSummary) Label() string {
	switch c.State {
	case CheckPassing:
		return "checks passing"
	case CheckFailing:
		if c.Failing > 0 {
			return fmt.Sprintf("checks failing (%d)", c.Failing)
		}
		return "checks failing"
	case CheckPending:
		if c.Pending > 0 {
			return fmt.Sprintf("checks pending (%d)", c.Pending)
		}
		return "checks pending"
	default:
		return "checks unknown"
	}
}

// SortByUpdatedDesc keeps the default PR list order deterministic.
func SortByUpdatedDesc(prs []PullRequest) {
	sort.SliceStable(prs, func(i, j int) bool {
		left, leftOK := parseUpdatedAt(prs[i].UpdatedAt)
		right, rightOK := parseUpdatedAt(prs[j].UpdatedAt)
		switch {
		case leftOK && rightOK:
			return left.After(right)
		case leftOK:
			return true
		case rightOK:
			return false
		default:
			return prs[i].Number > prs[j].Number
		}
	})
}

func parseUpdatedAt(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed, true
	}
	parsed, err = time.Parse("2006-01-02", value)
	return parsed, err == nil
}
