package workbench

import (
	"context"
	"fmt"
	"strings"
)

// PullRequestDiscovery provides pull request data for linking work items.
type PullRequestDiscovery interface {
	PullRequests(ctx context.Context, repo string) ([]PullRequestRef, error)
}

// PullRequestLinkService links GitHub pull requests onto local workbench items.
type PullRequestLinkService struct {
	GitHub PullRequestDiscovery
}

// LinkForRepo loads pull requests and links them to branch-backed work items.
func (s PullRequestLinkService) LinkForRepo(ctx context.Context, repo RepoRef, items []WorkItem) []WorkItem {
	if s.GitHub == nil {
		return cloneWorkItems(items)
	}
	prs, err := s.GitHub.PullRequests(ctx, repo.FullName())
	if err != nil {
		return append(cloneWorkItems(items), pullRequestDiscoveryErrorItem(repo, err))
	}
	return LinkPullRequests(items, prs)
}

// LinkPullRequests matches pull requests to work items by branch head.
func LinkPullRequests(items []WorkItem, prs []PullRequestRef) []WorkItem {
	byBranch := map[pullRequestBranchKey]PullRequestRef{}
	for _, pr := range prs {
		if pr.HeadBranch == "" {
			continue
		}
		key := pullRequestBranchKey{owner: normalizedOwner(pr.HeadOwner), branch: pr.HeadBranch}
		if existing, exists := byBranch[key]; !exists || preferredPullRequest(pr, existing) {
			byBranch[key] = pr
		}
	}

	out := cloneWorkItems(items)
	for i := range out {
		if out[i].Branch == nil || out[i].Branch.Name == "" {
			continue
		}
		out[i].PullRequest = nil
		pr, ok := byBranch[pullRequestBranchKey{owner: normalizedOwner(out[i].Repo.Owner), branch: out[i].Branch.Name}]
		if !ok {
			pr, ok = byBranch[pullRequestBranchKey{branch: out[i].Branch.Name}]
		}
		if !ok {
			continue
		}
		out[i].PullRequest = &pr
		if out[i].Checks.State == "" {
			out[i].Checks = CheckSummary{State: CheckUnknown}
		}
	}
	return out
}

type pullRequestBranchKey struct {
	owner  string
	branch string
}

func normalizedOwner(owner string) string {
	return strings.ToLower(owner)
}

func preferredPullRequest(candidate PullRequestRef, existing PullRequestRef) bool {
	return strings.EqualFold(candidate.State, "open") && !strings.EqualFold(existing.State, "open")
}

func cloneWorkItems(items []WorkItem) []WorkItem {
	return append([]WorkItem(nil), items...)
}

func pullRequestDiscoveryErrorItem(repo RepoRef, err error) WorkItem {
	return WorkItem{
		ID:     "pull-request-discovery-error:" + repo.FullName(),
		Repo:   repo,
		Branch: &BranchRef{Name: "pull request discovery error"},
		Local: &LocalStatus{
			State:   LocalUnknown,
			Summary: fmt.Sprintf("pull request discovery failed: %v", err),
		},
		Checks: CheckSummary{State: CheckUnknown},
	}
}
