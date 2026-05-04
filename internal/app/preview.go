package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0maru/gh-zen/internal/workbench"
)

const defaultPreviewDelay = 120 * time.Millisecond

type previewStatus int

const (
	previewIdle previewStatus = iota
	previewLoading
	previewLoaded
	previewEmpty
	previewError
)

type previewState struct {
	status            previewStatus
	requestID         int
	focusedWorkItemID string
	loaded            previewData
	errorMessage      string
}

type previewRequest struct {
	requestID  int
	workItemID string
	item       workbench.WorkItem
}

type previewData struct {
	workItemID string
	item       workbench.WorkItem
}

type previewResultMsg struct {
	requestID  int
	workItemID string
	data       previewData
	empty      bool
	err        error
}

type previewLoader func(previewRequest) tea.Cmd

func fakeDelayedPreviewLoader(delay time.Duration) previewLoader {
	return func(req previewRequest) tea.Cmd {
		return func() tea.Msg {
			if delay > 0 {
				time.Sleep(delay)
			}
			return previewResultMsg{
				requestID:  req.requestID,
				workItemID: req.workItemID,
				data: previewData{
					workItemID: req.workItemID,
					item:       req.item,
				},
			}
		}
	}
}

func workItemPreviewLines(item workbench.WorkItem, width int) []string {
	lines := []string{
		truncate("Repo: "+item.Repo.FullName(), width),
		truncate("Item: "+item.Title(), width),
		truncate("Where: "+item.Location(), width),
	}
	if item.Branch != nil {
		base := item.Branch.Base
		if base == "" {
			base = "unknown base"
		}
		lines = append(lines, truncate("Branch: "+item.Branch.Name+" -> "+base, width))
	}
	lines = append(lines, truncate("Local: "+item.LocalLabel(), width))
	lines = append(lines, truncate("Issue: "+item.IssueLabel(), width))
	lines = append(lines, truncate("PR: "+item.PullRequestLabel(), width))
	if item.PullRequest != nil {
		if head := item.PullRequest.HeadLabel(); head != "" {
			lines = append(lines, truncate("Head: "+head, width))
		}
		if item.PullRequest.AuthorLogin != "" {
			lines = append(lines, truncate("Author: "+item.PullRequest.AuthorLogin, width))
		}
		if reason := reviewReasonLabel(*item.PullRequest); reason != "" {
			lines = append(lines, truncate("Reason: "+reason, width))
		}
		if requested := reviewRequestSummary(item.PullRequest.ReviewRequests); requested != "" {
			lines = append(lines, truncate("Requested: "+requested, width))
		}
		if reviews := latestReviewSummary(item.PullRequest.LatestReviews); reviews != "" {
			lines = append(lines, truncate("Reviews: "+reviews, width))
		}
	}
	if item.PullRequest != nil && item.PullRequest.ReviewState != "" {
		lines = append(lines, truncate("Review: "+item.PullRequest.ReviewState, width))
	}
	lines = append(lines, truncate("Checks: "+item.Checks.Label(), width))
	if len(item.Commits) > 0 {
		lines = append(lines, "Commits:")
		for _, commit := range item.Commits {
			lines = append(lines, truncate("  "+commit.ShortSHA+" "+commit.Subject, width))
		}
	}
	return lines
}

func issueCertaintyLabel(issue workbench.IssueRef) string {
	if issue.Certain {
		return "certain"
	}
	return "heuristic"
}

func issueBodyExcerpt(body string) string {
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return ""
	}
	const maxWords = 28
	if len(fields) > maxWords {
		fields = fields[:maxWords]
		return strings.Join(fields, " ") + "..."
	}
	return strings.Join(fields, " ")
}

func reviewReasonLabel(pr workbench.PullRequestRef) string {
	switch {
	case pr.ViewerReviewRequested:
		return "needs your review"
	case pr.WaitingOnReview:
		return "waiting on review"
	default:
		return ""
	}
}

func reviewRequestSummary(requests []workbench.ReviewRequestRef) string {
	if len(requests) == 0 {
		return ""
	}
	labels := make([]string, 0, len(requests))
	for _, request := range requests {
		switch {
		case request.Login != "":
			labels = append(labels, request.Login)
		case request.Slug != "":
			labels = append(labels, "team/"+request.Slug)
		case request.Name != "":
			labels = append(labels, request.Name)
		}
	}
	return strings.Join(labels, ", ")
}

func latestReviewSummary(reviews []workbench.PullRequestReviewRef) string {
	if len(reviews) == 0 {
		return ""
	}
	labels := make([]string, 0, len(reviews))
	for _, review := range reviews {
		if review.AuthorLogin == "" && review.State == "" {
			continue
		}
		if review.AuthorLogin == "" {
			labels = append(labels, review.State)
			continue
		}
		if review.State == "" {
			labels = append(labels, review.AuthorLogin)
			continue
		}
		labels = append(labels, review.AuthorLogin+" "+review.State)
	}
	return strings.Join(labels, ", ")
}
