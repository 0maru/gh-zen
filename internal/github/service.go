package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/0maru/gh-zen/internal/workbench"
)

// Service is the GitHub data boundary consumed by application commands.
type Service interface {
	RepositorySummary(ctx context.Context, repo string) (RepositorySummary, error)
	PullRequests(ctx context.Context, repo string) ([]workbench.PullRequestRef, error)
	Issues(ctx context.Context, repo string) ([]workbench.IssueRef, error)
	CheckSummary(ctx context.Context, repo string, ref string) (workbench.CheckSummary, error)
}

// RepositorySummary contains lightweight GitHub data for a repository refresh.
type RepositorySummary struct {
	Repo         string
	PullRequests []workbench.PullRequestRef
	Issues       []workbench.IssueRef
	Checks       workbench.CheckSummary
}

// Runner executes gh commands.
type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// CLIService is a gh-backed GitHub service skeleton.
type CLIService struct {
	Runner Runner
}

// GHRunner executes the gh binary.
type GHRunner struct{}

// ErrorKind classifies gh failures into user-actionable categories.
type ErrorKind string

const (
	ErrorAuth    ErrorKind = "auth"
	ErrorNetwork ErrorKind = "network"
	ErrorCommand ErrorKind = "command"

	listLimit = "1000"
)

// Error describes a gh-backed service failure.
type Error struct {
	Op     string
	Kind   ErrorKind
	Output string
	Err    error
}

func (e Error) Error() string {
	output := strings.TrimSpace(e.Output)
	if output == "" {
		return fmt.Sprintf("%s failed (%s): %v", e.Op, e.Kind, e.Err)
	}
	return fmt.Sprintf("%s failed (%s): %s", e.Op, e.Kind, output)
}

func (e Error) Unwrap() error {
	return e.Err
}

// Run executes gh and returns raw command output.
func (GHRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isPendingChecksExit(args, err) {
			return output, nil
		}
		return nil, classifyError("gh "+strings.Join(args, " "), output, err)
	}
	return output, nil
}

// RepositorySummary loads lightweight PR and issue summaries.
func (s CLIService) RepositorySummary(ctx context.Context, repo string) (RepositorySummary, error) {
	prs, err := s.PullRequests(ctx, repo)
	if err != nil {
		return RepositorySummary{}, err
	}
	issues, err := s.Issues(ctx, repo)
	if err != nil {
		return RepositorySummary{}, err
	}
	return RepositorySummary{
		Repo:         repo,
		PullRequests: prs,
		Issues:       issues,
		Checks:       workbench.CheckSummary{State: workbench.CheckUnknown},
	}, nil
}

// PullRequests loads pull request summaries through gh.
func (s CLIService) PullRequests(ctx context.Context, repo string) ([]workbench.PullRequestRef, error) {
	output, err := s.runner().Run(ctx, "pr", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", "number,title,state,url,headRefName,headRepositoryOwner,reviewDecision")
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Number              int    `json:"number"`
		Title               string `json:"title"`
		State               string `json:"state"`
		URL                 string `json:"url"`
		HeadRefName         string `json:"headRefName"`
		HeadRepositoryOwner struct {
			Login string `json:"login"`
		} `json:"headRepositoryOwner"`
		ReviewDecision string `json:"reviewDecision"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse gh pr list output: %w", err)
	}
	prs := make([]workbench.PullRequestRef, 0, len(payload))
	for _, pr := range payload {
		prs = append(prs, workbench.PullRequestRef{
			Number:      pr.Number,
			Title:       pr.Title,
			State:       strings.ToLower(pr.State),
			URL:         pr.URL,
			HeadOwner:   pr.HeadRepositoryOwner.Login,
			HeadBranch:  pr.HeadRefName,
			ReviewState: reviewState(pr.ReviewDecision),
		})
	}
	return prs, nil
}

// Issues loads issue summaries through gh.
func (s CLIService) Issues(ctx context.Context, repo string) ([]workbench.IssueRef, error) {
	output, err := s.runner().Run(ctx, "issue", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", "number,title,state,url")
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse gh issue list output: %w", err)
	}
	issues := make([]workbench.IssueRef, 0, len(payload))
	for _, issue := range payload {
		issues = append(issues, workbench.IssueRef{
			Number:  issue.Number,
			Title:   issue.Title,
			State:   strings.ToLower(issue.State),
			URL:     issue.URL,
			Certain: true,
		})
	}
	return issues, nil
}

// CheckSummary loads a check summary for a pull request or branch ref through gh.
func (s CLIService) CheckSummary(ctx context.Context, repo string, ref string) (workbench.CheckSummary, error) {
	if ref == "" {
		return workbench.CheckSummary{State: workbench.CheckUnknown}, nil
	}
	output, err := s.runner().Run(ctx, "pr", "checks", ref, "--repo", repo, "--json", "name,state")
	if err != nil {
		return workbench.CheckSummary{}, err
	}
	var payload []struct {
		Name  string `json:"name"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return workbench.CheckSummary{}, fmt.Errorf("parse gh pr checks output: %w", err)
	}
	states := make([]string, 0, len(payload))
	for _, check := range payload {
		states = append(states, check.State)
	}
	return summarizeCheckStates(states), nil
}

func (s CLIService) runner() Runner {
	if s.Runner != nil {
		return s.Runner
	}
	return GHRunner{}
}

func isPendingChecksExit(args []string, err error) bool {
	if len(args) < 2 || args[0] != "pr" || args[1] != "checks" {
		return false
	}
	code, ok := exitCode(err)
	return ok && code == 8
}

type exitCoder interface {
	ExitCode() int
}

func exitCode(err error) (int, bool) {
	var exitErr exitCoder
	if !errors.As(err, &exitErr) {
		return 0, false
	}
	return exitErr.ExitCode(), true
}

func reviewState(value string) string {
	value = strings.ToLower(strings.ReplaceAll(value, "_", " "))
	return strings.TrimSpace(value)
}

func summarizeCheckStates(states []string) workbench.CheckSummary {
	summary := workbench.CheckSummary{State: workbench.CheckUnknown}
	for _, state := range states {
		switch normalizedCheckState(state) {
		case workbench.CheckPassing:
			summary.Passing++
		case workbench.CheckFailing:
			summary.Failing++
		case workbench.CheckPending:
			summary.Pending++
		}
	}
	switch {
	case summary.Failing > 0:
		summary.State = workbench.CheckFailing
	case summary.Pending > 0:
		summary.State = workbench.CheckPending
	case summary.Passing > 0:
		summary.State = workbench.CheckPassing
	}
	return summary
}

func normalizedCheckState(value string) workbench.CheckState {
	value = strings.ToLower(value)
	switch {
	case strings.Contains(value, "fail"), strings.Contains(value, "error"), strings.Contains(value, "cancel"):
		return workbench.CheckFailing
	case strings.Contains(value, "pending"), strings.Contains(value, "queued"), strings.Contains(value, "progress"), strings.Contains(value, "waiting"):
		return workbench.CheckPending
	case strings.Contains(value, "pass"), strings.Contains(value, "success"):
		return workbench.CheckPassing
	default:
		return workbench.CheckUnknown
	}
}

func classifyError(op string, output []byte, err error) Error {
	text := strings.ToLower(string(output))
	kind := ErrorCommand
	switch {
	case strings.Contains(text, "auth"), strings.Contains(text, "login"), strings.Contains(text, "credential"):
		kind = ErrorAuth
	case strings.Contains(text, "network"), strings.Contains(text, "resolve"), strings.Contains(text, "connection"), strings.Contains(text, "timeout"), strings.Contains(text, "tls"):
		kind = ErrorNetwork
	}
	return Error{Op: op, Kind: kind, Output: string(output), Err: err}
}
