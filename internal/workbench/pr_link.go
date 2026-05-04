package workbench

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// PullRequestDiscovery provides pull request data for linking work items.
type PullRequestDiscovery interface {
	PullRequests(ctx context.Context, repo string) ([]PullRequestRef, error)
}

// ReviewSubjectDiscovery provides the authenticated viewer and review team subjects.
type ReviewSubjectDiscovery interface {
	ViewerReviewSubjects(ctx context.Context) (ReviewSubjects, error)
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
	var discoveryErrors []error
	subjects, err := reviewSubjects(ctx, s.GitHub)
	if err != nil {
		discoveryErrors = append(discoveryErrors, err)
	}
	if !subjects.Empty() {
		prs = ApplyReviewPerspective(prs, subjects)
	}

	out := LinkPullRequestsForRepo(repo, items, prs)
	if len(discoveryErrors) > 0 {
		return append(out, pullRequestDiscoveryErrorItem(repo, errors.Join(discoveryErrors...)))
	}
	return out
}

// LinkPullRequests matches pull requests to work items by branch head.
func LinkPullRequests(items []WorkItem, prs []PullRequestRef) []WorkItem {
	repo := RepoRef{}
	for _, item := range items {
		if item.Repo != (RepoRef{}) {
			repo = item.Repo
			break
		}
	}
	return LinkPullRequestsForRepo(repo, items, prs)
}

// ApplyReviewPerspective marks PRs that are relevant to the authenticated viewer.
func ApplyReviewPerspective(prs []PullRequestRef, subjects ReviewSubjects) []PullRequestRef {
	out := append([]PullRequestRef(nil), prs...)
	for i := range out {
		out[i] = out[i].WithReviewPerspective(subjects)
	}
	return out
}

// LinkPullRequestsForRepo matches pull requests to local work items and appends PR-backed items.
func LinkPullRequestsForRepo(repo RepoRef, items []WorkItem, prs []PullRequestRef) []WorkItem {
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
	linkedPullRequests := map[string]bool{}
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
		linkedPullRequests[pullRequestIdentity(pr)] = true
		if out[i].Checks.State == "" {
			out[i].Checks = CheckSummary{State: CheckUnknown}
		}
	}
	appendedPullRequests := map[string]bool{}
	for _, pr := range prs {
		identity := pullRequestIdentity(pr)
		if linkedPullRequests[identity] || appendedPullRequests[identity] || !shouldCreatePullRequestWorkItem(pr) {
			continue
		}
		out = append(out, pullRequestWorkItem(repo, pr))
		appendedPullRequests[identity] = true
	}
	return out
}

func reviewSubjects(ctx context.Context, discovery PullRequestDiscovery) (ReviewSubjects, error) {
	subjectDiscovery, ok := discovery.(ReviewSubjectDiscovery)
	if !ok {
		return ReviewSubjects{}, nil
	}
	subjects, err := subjectDiscovery.ViewerReviewSubjects(ctx)
	if err != nil {
		return subjects, fmt.Errorf("viewer review subject discovery failed: %w", err)
	}
	return subjects, nil
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

func shouldCreatePullRequestWorkItem(pr PullRequestRef) bool {
	return strings.EqualFold(pr.State, "open") && (pr.Number > 0 || pr.HeadBranch != "")
}

func pullRequestWorkItem(repo RepoRef, pr PullRequestRef) WorkItem {
	item := WorkItem{
		ID:          "pull-request:" + repo.FullName() + ":" + pullRequestIdentity(pr),
		Repo:        repo,
		PullRequest: &pr,
		Checks:      CheckSummary{State: CheckUnknown},
		Local: &LocalStatus{
			State:   LocalMissing,
			Summary: pullRequestOnlyLocalSummary(repo, pr),
		},
	}
	if pr.HeadBranch != "" {
		item.Branch = &BranchRef{
			Name:       pr.HeadBranch,
			Base:       pr.BaseBranch,
			RemoteOnly: true,
		}
	}
	return item
}

func pullRequestIdentity(pr PullRequestRef) string {
	if pr.Number > 0 {
		return fmt.Sprintf("#%d", pr.Number)
	}
	if pr.HeadBranch == "" {
		return "unknown"
	}
	owner := normalizedOwner(pr.HeadOwner)
	if owner == "" {
		return pr.HeadBranch
	}
	return owner + ":" + pr.HeadBranch
}

func pullRequestOnlyLocalSummary(repo RepoRef, pr PullRequestRef) string {
	if pr.HeadOwner != "" && repo.Owner != "" && !strings.EqualFold(pr.HeadOwner, repo.Owner) {
		return "no local worktree; fork head " + pr.HeadOwner
	}
	return "no local worktree"
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
