# 0006: Use Repository Workbench as the Primary Navigation Model

- Status: Accepted
- Date: 2026-04-26
- Supersedes: [0003: Use Yazi-Inspired Object Navigation and Previews](0003-use-yazi-inspired-object-navigation-and-previews.md)

## Context

ADR 0003 established a Yazi-inspired interaction model and proposed a
current-repository pull request browser as the first product slice. That preview
and navigation direction still fits `gh-zen`, but a pull request browser is too
late in the workflow for the way this project should support CodingAgents and
parallel development.

The common workflow is closer to:

```text
issue -> agent branch -> local worktree -> commits -> pull request -> checks -> review -> merge
```

When multiple agents or terminal sessions are working at the same time, users
need to answer broader questions than "which pull request should I open?":

- Which repositories are active?
- Which local worktrees exist for this repository?
- Which branch is each worktree on?
- Which branches have pull requests?
- Which branches are remote-only and have no local worktree yet?
- Which issues are linked to active branches or pull requests?
- Which work is dirty, blocked, failing checks, or ready for review?

This makes the repository, local worktree, and branch the natural starting point
for navigation. Pull requests and issues remain important, but they are context
attached to a broader unit of work.

## Decision

Use a repository workbench as the primary navigation model.

The primary object in the center list is a repo-scoped work item. A work item is
not only a pull request. It may combine local and remote state:

- Repository.
- Local worktree, when one exists.
- Branch, local branch, or remote-only branch.
- Linked issue, when discoverable.
- Linked pull request, when one exists.
- Recent commits.
- Check and review summary.
- Local status, such as clean, dirty, missing, or detached.

The default layout should grow toward:

- Left: repositories, repository groups, saved views, and filters.
- Middle: work items for the selected repository or view.
- Right: preview and actions for the focused work item.

Worktrees are first-class state. A worktree is the local place where a human or
agent is doing work, so it should be visible beside the branch, issue, pull
request, and check state. Remote-only branches and issue-only work can still
appear in the same model as work items without local worktrees.

The center list should make local and remote state easy to compare. Example
rows:

```text
~/src/gh-zen              main                     clean
~/src/gh-zen-agent-a      feat/config-loader       dirty   PR #12
~/src/gh-zen-agent-b      feat/repo-workbench      clean   no PR
remote only               agent/preview-state      pushed  no PR
issue only                #34 Branch preview UX    unstarted
```

The right preview should show the focused work item's combined context:

- Worktree path and local status.
- Branch name, base branch, and ahead/behind summary when available.
- Linked issue title and status.
- Linked pull request state, review status, and check summary.
- Recent commits.
- Relevant actions.

The Yazi-inspired behavior from ADR 0003 still applies:

- Cursor movement should be cheap and continuous.
- Focusing a work item should start loading its preview.
- Preview loading should be asynchronous and discardable.
- Stale preview responses must not replace the current preview.
- Direct keyboard movement and contextual actions should remain preferred over
  modal page transitions.

The first concrete product slice should be a fake-data repository workbench:

- Left: a repository and saved-view list.
- Middle: fake work items with a mix of worktree, branch, issue, pull request,
  and check states.
- Right: fake work item preview.

This slice should prove the layout, work item model, cursor movement, preview
state, and key help before real GitHub or Git data is connected.

## Consequences

Positive:

- The primary UI matches parallel CodingAgents and multi-worktree development.
- Pull requests, issues, branches, checks, and local status can be inspected in
  one place.
- Work that has not reached pull request stage is still visible.
- The UI can support both local operations and GitHub operations from the same
  focused item.
- ADR 0003's fast preview and stale-result discard model remains useful without
  forcing PRs to be the starting point.

Tradeoffs:

- The work item model is more complex than a pull request list.
- Linking branches, worktrees, issues, and pull requests requires heuristics and
  explicit data boundaries.
- Local Git state and remote GitHub state may disagree and must be represented
  without hiding conflicts.
- Some views may contain issue-only or pull-request-only items without a local
  worktree, so the UI cannot assume every row has a path.

## Implementation Notes

Keep the first implementation fake and deterministic. Suggested initial model:

```go
type WorkItem struct {
	Repo        RepoRef
	Branch      *BranchRef
	Worktree    *WorktreeRef
	Issue       *IssueRef
	PullRequest *PullRequestRef
	Checks      CheckSummary
	Local       *LocalStatus
}
```

The exact fields may evolve, but implementation should preserve these
boundaries:

- Git and filesystem discovery should live behind a local repository service.
- GitHub API or `gh` calls should live behind a GitHub service.
- The Bubble Tea model should consume work item data through interfaces or
  commands, not call Git or GitHub directly from rendering.
- View rendering should tolerate missing branch, worktree, issue, pull request,
  and check data.

Git worktree discovery should use parseable Git output:

```sh
git worktree list --porcelain
```

Future local repository discovery may also use:

```sh
git rev-parse --show-toplevel
git remote get-url origin
git status --porcelain=v1
```

Initial actions should be read-only:

- Open pull request in browser.
- Open issue in browser.
- Copy URL.
- Copy worktree path.
- Refresh.

Mutating actions such as creating a worktree, removing a worktree, creating a
pull request, assigning labels, or closing issues should come later with
confirmation and explicit error handling.

Testing should follow ADR 0004:

- Small tests cover work item modeling, key handling, preview stale discard, and
  deterministic views.
- Medium tests cover temporary Git repositories and fake command boundaries.
- Large tests cover real GitHub API, real `gh` auth, or browser behavior.

## Alternatives Considered

- Keep pull requests as the primary list: simpler, but it hides branches and
  worktrees before PR creation.
- Use issues as the primary list: useful for planning, but weaker for local
  branch, worktree, and check-state operations.
- Use local worktrees only: good for local agent coordination, but it misses
  remote-only branches, PR-only state, and issue-only work.
- Make repositories the only list: useful for switching context, but users still
  need a repo-scoped unit that combines local and GitHub work state.

## Maintenance Notes

This ADR should be revisited when:

- Work items stop being the primary navigation object.
- Worktrees are no longer first-class in the UI.
- Pull requests or issues become the primary list again.
- The application no longer targets parallel CodingAgents or multi-worktree
  workflows.

If the navigation model changes materially, add a new ADR and mark this one as
`Superseded` instead of rewriting this decision in place.
