package workbench

import (
	"context"
	"fmt"
	"sort"

	"github.com/0maru/gh-zen/internal/localrepo"
)

// LocalDiscovery provides local Git state for workbench item assembly.
type LocalDiscovery interface {
	DiscoverWorktrees(ctx context.Context, repoPath string) ([]localrepo.Worktree, error)
	DiscoverBranches(ctx context.Context, repoPath string) ([]localrepo.Branch, error)
}

// LocalWorkItemService builds workbench items from local Git state.
type LocalWorkItemService struct {
	Repo      RepoRef
	RepoPath  string
	Discovery LocalDiscovery
}

// WorkItems returns workbench items for local worktrees and branch refs.
func (s LocalWorkItemService) WorkItems(ctx context.Context) []WorkItem {
	discovery := s.Discovery
	if discovery == nil {
		discovery = localrepo.Service{}
	}

	worktrees, err := discovery.DiscoverWorktrees(ctx, s.RepoPath)
	if err != nil {
		return []WorkItem{localDiscoveryErrorItem(s.Repo, err)}
	}
	branches, err := discovery.DiscoverBranches(ctx, s.RepoPath)
	if err != nil {
		return []WorkItem{localDiscoveryErrorItem(s.Repo, err)}
	}

	return AssembleLocalWorkItems(s.Repo, worktrees, branches)
}

// AssembleLocalWorkItems converts local discovery data into workbench work items.
func AssembleLocalWorkItems(repo RepoRef, worktrees []localrepo.Worktree, branches []localrepo.Branch) []WorkItem {
	items := make([]WorkItem, 0, len(worktrees)+len(branches))
	seenBranches := map[string]bool{}

	sort.SliceStable(worktrees, func(i, j int) bool {
		return worktrees[i].Path < worktrees[j].Path
	})
	for _, worktree := range worktrees {
		if worktree.Branch != "" {
			seenBranches[worktree.Branch] = true
		}
		items = append(items, worktreeWorkItem(repo, worktree))
	}

	localBranches, remoteBranches := splitBranches(branches)
	for _, branch := range localBranches {
		if seenBranches[branch.Name] {
			continue
		}
		seenBranches[branch.Name] = true
		items = append(items, branchWorkItem(repo, branch))
	}
	for _, branch := range remoteBranches {
		if seenBranches[branch.Name] {
			continue
		}
		seenBranches[branch.Name] = true
		items = append(items, branchWorkItem(repo, branch))
	}

	return items
}

func splitBranches(branches []localrepo.Branch) ([]localrepo.Branch, []localrepo.Branch) {
	localBranches := []localrepo.Branch{}
	remoteBranches := []localrepo.Branch{}
	for _, branch := range branches {
		if branch.Name == "" {
			continue
		}
		if branch.RemoteOnly {
			remoteBranches = append(remoteBranches, branch)
			continue
		}
		localBranches = append(localBranches, branch)
	}
	sortBranches(localBranches)
	sortBranches(remoteBranches)
	return localBranches, remoteBranches
}

func sortBranches(branches []localrepo.Branch) {
	sort.SliceStable(branches, func(i, j int) bool {
		if branches[i].Name == branches[j].Name {
			return branches[i].Remote < branches[j].Remote
		}
		return branches[i].Name < branches[j].Name
	})
}

func worktreeWorkItem(repo RepoRef, worktree localrepo.Worktree) WorkItem {
	item := WorkItem{
		ID:       "worktree:" + worktree.Path,
		Repo:     repo,
		Worktree: &WorktreeRef{Path: worktree.Path},
		Local:    localStatus(worktree),
	}
	if worktree.Branch != "" {
		item.Branch = &BranchRef{Name: worktree.Branch}
	}
	return item
}

func branchWorkItem(repo RepoRef, branch localrepo.Branch) WorkItem {
	item := WorkItem{
		ID:     "branch:" + branch.Name,
		Repo:   repo,
		Branch: &BranchRef{Name: branch.Name, RemoteOnly: branch.RemoteOnly},
		Local:  &LocalStatus{State: LocalUnknown},
	}
	if branch.RemoteOnly {
		item.ID = "remote:" + branch.Remote + "/" + branch.Name
		item.Local = &LocalStatus{State: LocalMissing, Summary: "no local worktree"}
	}
	return item
}

func localStatus(worktree localrepo.Worktree) *LocalStatus {
	switch {
	case worktree.Missing:
		return &LocalStatus{State: LocalMissing, Summary: missingSummary(worktree)}
	case worktree.Detached:
		return &LocalStatus{State: LocalDetached, Summary: statusSummary(worktree)}
	case worktree.Dirty:
		return &LocalStatus{State: LocalDirty, Summary: statusSummary(worktree)}
	default:
		return &LocalStatus{State: LocalClean}
	}
}

func missingSummary(worktree localrepo.Worktree) string {
	if worktree.PrunableReason != "" {
		return "prunable: " + worktree.PrunableReason
	}
	if worktree.Prunable {
		return "prunable"
	}
	return "missing worktree"
}

func statusSummary(worktree localrepo.Worktree) string {
	if len(worktree.StatusEntries) == 0 {
		return ""
	}
	if len(worktree.StatusEntries) == 1 {
		return "1 status entry"
	}
	return fmt.Sprintf("%d status entries", len(worktree.StatusEntries))
}

func localDiscoveryErrorItem(repo RepoRef, err error) WorkItem {
	return WorkItem{
		ID:     "local-discovery-error:" + repo.FullName(),
		Repo:   repo,
		Branch: &BranchRef{Name: "local discovery error"},
		Local: &LocalStatus{
			State:   LocalUnknown,
			Summary: "local discovery failed: " + err.Error(),
		},
	}
}
