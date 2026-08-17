# 0010: Use Terminal-First GitHub Browsing as the Product Direction

- Status: Accepted
- Date: 2026-06-21
- Related: [0003: Use Yazi-Inspired Object Navigation and Previews](0003-use-yazi-inspired-object-navigation-and-previews.md)
- Related: [0006: Use Repository Workbench as the Primary Navigation Model](0006-use-repository-workbench-as-the-primary-navigation-model.md)
- Related: [0009: Use a Runtime Data Pipeline for the Repository Workbench](0009-use-a-runtime-data-pipeline-for-the-repository-workbench.md)

## Context

The repository workbench remains the primary navigation model for parallel local
work: worktrees, branches, linked issues, pull requests, checks, and review state
are easier to understand as one repo-scoped work item.

That model does not fully replace GitHub object browsing. Users still need a
terminal-native way to scan all pull requests in the current repository,
including PRs that are not tied to a local branch or worktree, closed and merged
PRs, drafts, review-requested work, failed checks, and text-search matches.
Opening a browser or running `gh pr list` separately breaks the product loop that
`gh-zen` is building.

## Decision

`gh-zen` should provide terminal-first GitHub browsing as a product direction,
starting with a first-class current-repository pull request browser.

The repository workbench is still the default view. The pull request browser is a
parallel view, not a replacement:

- Workbench view: repo-scoped work items that combine local worktree state and
  GitHub state.
- Pull request view: all pull requests for the selected repository, sorted by
  `updatedAt` descending, with filterable list rows and an asynchronous preview.

Both views should use the same product principles:

- Terminal-native, keyboard-first navigation.
- Direct movement and contextual actions.
- GitHub IO behind service boundaries, not view rendering.
- Async preview loading with request identity and stale-result discard.
- Read-only browsing and launcher/copy actions unless a later ADR accepts
  mutation workflows.

The pull request browser should include enough data to avoid a browser round
trip for triage:

- PR number, title, state, draft status, author, head and base refs.
- Review decision, requested reviewers, latest reviews, and linked issues.
- Mergeability, updated time, body excerpt, and check summary.
- URL, PR number, and head-ref copy/open actions.

Filters should be local and deterministic once data is loaded. The first filter
set is lifecycle state, author, review-requested, waiting-on-review, failed
checks, draft status, and text search.

## Consequences

Positive:

- Users can inspect repository PRs without leaving the terminal.
- PR-only work no longer has to be forced into the workbench model to be visible.
- ADR 0003's object browser and preview design is restored without undoing ADR
  0006's workbench direction.
- The GitHub service boundary from ADR 0009 becomes reusable beyond workbench
  enrichment.

Tradeoffs:

- The application now has two first-class repo views, so key bindings and help
  must make the active mode explicit.
- Pull request fetching may become expensive on large repositories unless
  pagination, caching, or incremental refreshes are improved later.
- Preview and filter behavior must stay consistent across workbench and PR
  browsing to avoid separate interaction models.

## Implementation Notes

Keep the first implementation read-only:

- Add a dedicated `internal/pullrequests` package for PR browser types and pure
  filter logic.
- Keep GitHub fetching in `internal/github`; the app consumes a service
  interface.
- Preserve workbench PR binding and PR-only work items.
- Use a separate PR preview request ID and focused PR key.
- Add tests for filter combinations, state transitions, stale preview discard,
  key actions, view rendering, and fake GitHub command parsing.

## Alternatives Considered

- Replace the workbench with a pull request list: rejected because local
  multi-worktree coordination remains the primary workflow.
- Keep using `gh pr list` outside the app: rejected because it breaks browsing
  continuity and preview context.
- Add only another workbench saved view: rejected because workbench filters are
  work-item filters, not complete PR object filters.

## Maintenance Notes

Revisit this ADR when:

- The app adds another first-class GitHub object browser.
- Pull request browsing becomes mutating rather than read-only.
- A cross-repository browsing model replaces current-repository views.
- Pagination or caching changes the PR data contract materially.
