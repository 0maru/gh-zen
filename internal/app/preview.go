package app

import (
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
