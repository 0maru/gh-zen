package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/0maru/gh-zen/internal/workbench"
)

// Service is the GitHub data boundary consumed by application commands.
type Service interface {
	RepositorySummary(ctx context.Context, repo string) (RepositorySummary, error)
	PullRequests(ctx context.Context, repo string) ([]workbench.PullRequestRef, error)
	Issues(ctx context.Context, repo string) ([]workbench.IssueRef, error)
	CheckSummary(ctx context.Context, repo string, ref string) (workbench.CheckSummary, error)
	ViewerReviewSubjects(ctx context.Context) (workbench.ReviewSubjects, error)
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

	issueListFields = "number,title,state,url,body,labels,assignees,milestone,updatedAt"
	listLimit       = "1000"
	prListFields    = "number,title,state,url,headRefName,headRepositoryOwner,baseRefName,isDraft,updatedAt,author,reviewRequests,latestReviews,reviewDecision,body"
)

var (
	closingIssueTextPattern = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\b[^\n\r.]*`)
	issueNumberPattern      = regexp.MustCompile(`#(\d+)`)
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
	output, err := s.runner().Run(ctx, "pr", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", prListFields)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Number              int    `json:"number"`
		Title               string `json:"title"`
		State               string `json:"state"`
		URL                 string `json:"url"`
		HeadRefName         string `json:"headRefName"`
		BaseRefName         string `json:"baseRefName"`
		IsDraft             bool   `json:"isDraft"`
		UpdatedAt           string `json:"updatedAt"`
		Body                string `json:"body"`
		Author              ghUser `json:"author"`
		HeadRepositoryOwner struct {
			Login string `json:"login"`
		} `json:"headRepositoryOwner"`
		ReviewDecision string             `json:"reviewDecision"`
		ReviewRequests []ghReviewRequest  `json:"reviewRequests"`
		LatestReviews  []ghPullReviewItem `json:"latestReviews"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse gh pr list output: %w", err)
	}
	prs := make([]workbench.PullRequestRef, 0, len(payload))
	for _, pr := range payload {
		prs = append(prs, workbench.PullRequestRef{
			Number:         pr.Number,
			Title:          pr.Title,
			State:          strings.ToLower(pr.State),
			URL:            pr.URL,
			AuthorLogin:    pr.Author.Login,
			HeadOwner:      pr.HeadRepositoryOwner.Login,
			HeadBranch:     pr.HeadRefName,
			BaseBranch:     pr.BaseRefName,
			IsDraft:        pr.IsDraft,
			UpdatedAt:      pr.UpdatedAt,
			LinkedIssues:   linkedIssuesFromBody(pr.Body),
			ReviewState:    reviewState(pr.ReviewDecision),
			ReviewRequests: reviewRequests(pr.ReviewRequests),
			LatestReviews:  latestReviews(pr.LatestReviews),
		})
	}
	return prs, nil
}

// Issues loads issue summaries through gh.
func (s CLIService) Issues(ctx context.Context, repo string) ([]workbench.IssueRef, error) {
	output, err := s.runner().Run(ctx, "issue", "list", "--repo", repo, "--state", "all", "--limit", listLimit, "--json", issueListFields)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		State     string    `json:"state"`
		URL       string    `json:"url"`
		Body      string    `json:"body"`
		Labels    []ghLabel `json:"labels"`
		Assignees []ghUser  `json:"assignees"`
		Milestone *struct {
			Title string `json:"title"`
		} `json:"milestone"`
		UpdatedAt string `json:"updatedAt"`
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

// ViewerReviewSubjects loads the authenticated viewer and team slugs used for review request matching.
func (s CLIService) ViewerReviewSubjects(ctx context.Context) (workbench.ReviewSubjects, error) {
	output, err := s.runner().Run(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return workbench.ReviewSubjects{}, err
	}
	subjects := workbench.ReviewSubjects{Login: strings.TrimSpace(string(output))}

	output, err = s.runner().Run(ctx, "api", "user/teams", "--jq", ".[].slug")
	if err != nil {
		return subjects, err
	}
	subjects.TeamSlugs = nonEmptyLines(string(output))
	return subjects, nil
}

func (s CLIService) runner() Runner {
	if s.Runner != nil {
		return s.Runner
	}
	return GHRunner{}
}

type ghUser struct {
	Login string `json:"login"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghReviewRequest struct {
	TypeName string `json:"__typename"`
	Login    string `json:"login"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
}

type ghPullReviewItem struct {
	Author ghUser `json:"author"`
	State  string `json:"state"`
}

func linkedIssuesFromBody(body string) []workbench.IssueRef {
	seen := map[int]bool{}
	issues := []workbench.IssueRef{}
	for _, text := range closingIssueTextPattern.FindAllString(body, -1) {
		for _, match := range issueNumberPattern.FindAllStringSubmatch(text, -1) {
			if len(match) < 2 {
				continue
			}
			number, err := strconv.Atoi(match[1])
			if err != nil || seen[number] {
				continue
			}
			issues = append(issues, workbench.IssueRef{
				Number:  number,
				Certain: true,
			})
			seen[number] = true
		}
	}
	return issues
}

func labelNames(labels []ghLabel) []string {
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		if label.Name != "" {
			names = append(names, label.Name)
		}
	}
	return names
}

func userLogins(users []ghUser) []string {
	logins := make([]string, 0, len(users))
	for _, user := range users {
		if user.Login != "" {
			logins = append(logins, user.Login)
		}
	}
	return logins
}

func milestoneTitle(milestone *struct {
	Title string `json:"title"`
}) string {
	if milestone == nil {
		return ""
	}
	return milestone.Title
}

func reviewRequests(payload []ghReviewRequest) []workbench.ReviewRequestRef {
	requests := make([]workbench.ReviewRequestRef, 0, len(payload))
	for _, request := range payload {
		requests = append(requests, workbench.ReviewRequestRef{
			Kind:  request.TypeName,
			Login: request.Login,
			Name:  reviewRequestName(request),
			Slug:  request.Slug,
		})
	}
	return requests
}

func reviewRequestName(request ghReviewRequest) string {
	switch {
	case request.Name != "":
		return request.Name
	case request.Slug != "":
		return request.Slug
	default:
		return request.Login
	}
}

func latestReviews(payload []ghPullReviewItem) []workbench.PullRequestReviewRef {
	reviews := make([]workbench.PullRequestReviewRef, 0, len(payload))
	for _, review := range payload {
		reviews = append(reviews, workbench.PullRequestReviewRef{
			AuthorLogin: review.Author.Login,
			State:       reviewState(review.State),
		})
	}
	return reviews
}

func nonEmptyLines(value string) []string {
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
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
