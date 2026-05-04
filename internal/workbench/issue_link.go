package workbench

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	prefixedIssuePattern = regexp.MustCompile(`(?i)(?:^|[/_-])(?:issue|fix|fixes|gh|close|closes)[/_-]?(\d+)(?:$|[/_-])`)
	branchNumberPattern  = regexp.MustCompile(`\d+`)
)

// IssueCheckDiscovery provides issue and check data for work item enrichment.
type IssueCheckDiscovery interface {
	Issues(ctx context.Context, repo string) ([]IssueRef, error)
	CheckSummary(ctx context.Context, repo string, ref string) (CheckSummary, error)
}

// IssueCheckLinkService links issues and checks onto workbench items.
type IssueCheckLinkService struct {
	GitHub IssueCheckDiscovery
}

// LinkForRepo loads issues and check summaries, then enriches work items.
func (s IssueCheckLinkService) LinkForRepo(ctx context.Context, repo RepoRef, items []WorkItem) []WorkItem {
	if s.GitHub == nil {
		return cloneWorkItems(items)
	}
	var discoveryErrors []error
	issues, err := s.GitHub.Issues(ctx, repo.FullName())
	if err != nil {
		discoveryErrors = append(discoveryErrors, err)
	}

	out := LinkIssues(items, issues)
	for i := range out {
		if out[i].PullRequest == nil {
			continue
		}
		ref := pullRequestCheckRef(out[i])
		if ref == "" {
			continue
		}
		checks, err := s.GitHub.CheckSummary(ctx, repo.FullName(), ref)
		if err != nil {
			discoveryErrors = append(discoveryErrors, err)
			if out[i].Checks.State == "" {
				out[i].Checks = CheckSummary{State: CheckUnknown}
			}
			continue
		}
		if checks.State != "" {
			out[i].Checks = checks
		}
	}
	if len(discoveryErrors) > 0 {
		return append(out, issueCheckDiscoveryErrorItem(repo, errors.Join(discoveryErrors...)))
	}
	return out
}

// LinkIssues fills missing issue refs from PR metadata or branch naming.
func LinkIssues(items []WorkItem, issues []IssueRef) []WorkItem {
	byNumber := map[int]IssueRef{}
	for _, issue := range issues {
		if issue.Number > 0 {
			byNumber[issue.Number] = issue
		}
	}

	out := cloneWorkItems(items)
	for i := range out {
		if out[i].Issue != nil {
			continue
		}
		if issue, ok := issueFromPullRequest(out[i].PullRequest); ok {
			out[i].Issue = enrichIssue(issue, byNumber)
			continue
		}
		if out[i].Branch == nil {
			continue
		}
		if issue, ok := InferIssueFromBranch(out[i].Branch.Name); ok {
			out[i].Issue = enrichIssue(issue, byNumber)
		}
	}
	return out
}

// InferIssueFromBranch returns a possible issue reference from a branch name.
func InferIssueFromBranch(branch string) (IssueRef, bool) {
	prefixedNumbers := prefixedIssueNumbers(branch)
	if len(prefixedNumbers) > 0 {
		return IssueRef{Number: prefixedNumbers[0], Certain: len(prefixedNumbers) == 1}, true
	}

	numbers := branchNumberPattern.FindAllString(branch, -1)
	if len(numbers) != 1 {
		return IssueRef{}, false
	}
	number, err := strconv.Atoi(numbers[0])
	if err != nil {
		return IssueRef{}, false
	}
	return IssueRef{Number: number, Certain: false}, true
}

func prefixedIssueNumbers(branch string) []int {
	var numbers []int
	for start := 0; start < len(branch); {
		loc := prefixedIssuePattern.FindStringSubmatchIndex(branch[start:])
		if loc == nil {
			break
		}
		number, err := strconv.Atoi(branch[start+loc[2] : start+loc[3]])
		if err == nil {
			numbers = append(numbers, number)
		}
		next := start + loc[1]
		if next <= start {
			start++
			continue
		}
		start = next - 1
	}
	return numbers
}

func issueFromPullRequest(pr *PullRequestRef) (IssueRef, bool) {
	if pr == nil || len(pr.LinkedIssues) == 0 {
		return IssueRef{}, false
	}
	issue := pr.LinkedIssues[0]
	issue.Certain = len(pr.LinkedIssues) == 1
	return issue, true
}

func enrichIssue(issue IssueRef, byNumber map[int]IssueRef) *IssueRef {
	if known, ok := byNumber[issue.Number]; ok {
		known.Certain = issue.Certain
		return &known
	}
	return &issue
}

func pullRequestCheckRef(item WorkItem) string {
	if item.PullRequest == nil {
		return ""
	}
	if strings.HasPrefix(item.ID, "pull-request:") && item.PullRequest.Number > 0 {
		return strconv.Itoa(item.PullRequest.Number)
	}
	return item.PullRequest.HeadBranch
}

func issueCheckDiscoveryErrorItem(repo RepoRef, err error) WorkItem {
	return WorkItem{
		ID:     "issue-check-discovery-error:" + repo.FullName(),
		Repo:   repo,
		Branch: &BranchRef{Name: "issue and check discovery error"},
		Local: &LocalStatus{
			State:   LocalUnknown,
			Summary: fmt.Sprintf("issue and check discovery failed: %v", err),
		},
		Checks: CheckSummary{State: CheckUnknown},
	}
}
