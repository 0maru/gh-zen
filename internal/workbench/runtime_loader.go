package workbench

import "context"

// GitHubWorkbenchDiscovery provides GitHub data needed by the runtime workbench loader.
type GitHubWorkbenchDiscovery interface {
	PullRequestDiscovery
	IssueCheckDiscovery
}

// RuntimeLoadResult contains workbench data loaded for one repository.
type RuntimeLoadResult struct {
	Repo  RepoRef
	Items []WorkItem
}

// RuntimeLoader composes local Git discovery with GitHub workbench enrichment.
type RuntimeLoader struct {
	Repo     RepoRef
	RepoPath string
	Local    LocalDiscovery
	GitHub   GitHubWorkbenchDiscovery
}

// Load returns workbench items for one repository without failing on partial GitHub discovery errors.
func (l RuntimeLoader) Load(ctx context.Context) RuntimeLoadResult {
	items := (LocalWorkItemService{
		Repo:      l.Repo,
		RepoPath:  l.RepoPath,
		Discovery: l.Local,
	}).WorkItems(ctx)

	items = (PullRequestLinkService{
		GitHub: l.GitHub,
	}).LinkForRepo(ctx, l.Repo, items)
	items = (IssueCheckLinkService{
		GitHub: l.GitHub,
	}).LinkForRepo(ctx, l.Repo, items)

	return RuntimeLoadResult{
		Repo:  l.Repo,
		Items: items,
	}
}
