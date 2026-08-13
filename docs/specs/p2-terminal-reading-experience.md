# P2 Terminal Reading Experience

## Context

This specification turns the P2 terminal reading requirements into a shared
guide for issue, pull request, and Actions browsing work. It follows the
terminal-first direction described by
[ADR 0010](https://github.com/0maru/gh-zen/blob/93bf19c0e2ee19c76081d8082e78ed72d833a745/docs/adr/0010-use-terminal-first-github-browsing-as-the-product-direction.md)
and the existing repository boundaries from ADR 0009:

- `internal/github` owns `gh` command execution, GitHub pagination mechanics,
  and GitHub-specific error classification.
- `internal/app` owns Bubble Tea state, selection, commands, rendering, and
  stale-result discard.
- Domain packages such as `internal/workbench` own pure data shaping, filter,
  and sort logic.

ADR 0010 is currently carried by the pending pull request that introduces the
first-class pull request browser, rather than by `origin/main`. The commit link
above keeps the dependency verifiable without duplicating or revising the ADR
in this change.

The current baseline still uses the workbench panes (`paneWorkItems`,
`paneRepositories`, and `panePreview`) for focus, but it already has a
first-class Actions mode backed by `modeActions`, `actionsState`, and the
Actions methods on `internal/github.Service`. Dedicated Issues and Pull
Requests list views are not yet part of the baseline. This issue does not add
panes, app state, service interfaces, or GitHub API calls.

The phase 3, phase 4, and phase 5 labels below are rollout targets used by this
specification and its source planning issue. They are not numbered sections in
ADR 0010.

## Phase Targets

Phase 3, Issues:

- Establish the shared list state.
- Add first-page loading, issue sorting, issue search, and issue body
  truncation.
- Add consistent empty, loading, partial-error, and stale-data rendering.

Phase 4, Pull Requests:

- Reuse the shared list state for PR-specific sort and attention states.
- Cover PR body previews, linked issue context, review state, checks, and local
  filtering.

Phase 5, Actions / logs:

- Extend the existing Actions mode with the same patterns for workflow runs and
  check/log status; do not create a second Actions browser.
- Add large log pagination, explicit log expansion, and partial failures from
  run or log loading.

The implementation order should avoid adding a shared abstraction before one
view proves the shape. Phase 3 should still name the state and data contracts so
phase 4 and phase 5 can reuse them without changing behavior for users.

## Shared Reading List Model

Each dedicated reading view should be backed by a state shape equivalent to the
following, even if the first implementation keeps the types local to the view:

```go
type ReadingListState[T any] struct {
	Repo       workbench.RepoRef
	Items      []T
	Pages      []ReadingPage[T]
	NextCursor string
	Loading    bool
	Stale      bool
	PartialErr error
	FetchedAt  time.Time
	Selected   int
	Sort       ReadingSortMode
	Query      string
}
```

The exact type names may differ. The important contract is that list movement,
sort, and search operate on already loaded memory, while network work is
represented as asynchronous commands with request identity. A view may show
loaded data and a stale or partial-error banner at the same time.

## Pagination / Incremental Loading

### Policy

Initial view focus should load a bounded first chunk instead of all remote data.
The default chunk should be large enough for useful scanning but small enough to
keep `gh` commands fast:

- Issues and PRs: start with 50 rows, then load 50-row chunks.
- Workflow runs: start with 50 rows, then load 50-row chunks.
- Logs: start with the first relevant truncated segment, then load more log
  lines only through explicit expansion.

`internal/github` should expose opaque paging tokens to the app. The backing
implementation can use `gh issue list --limit`, `gh pr list --limit`, or
`gh run list --limit` for simple first slices, but cursor-aware loading should
move behind `gh api` calls when a command does not expose resumable cursors.
The app must not know whether the token is a GraphQL cursor, REST page number,
or empty end marker.

Loaded chunks should stay in memory per remote dataset key:

- Cache key: repository, view kind, lifecycle filter, and any ordering or query
  input that is actually sent to GitHub.
- Chunk content: items, fetched time, next cursor, and page-level error state.
- Staleness: mark cached data stale after five minutes, after an explicit
  refresh request, or after a remote dataset key change.

Accepted text search and local sort mode belong to view state layered on top of
the cached pages. Changing either must filter or reorder loaded memory without
invalidating pages or starting network work. If a later sort or search mode is
implemented remotely, only that explicitly remote mode becomes part of the
dataset key.

Incremental fetches should trigger when the focused row enters the last 20% of
the loaded list and `NextCursor` is non-empty. `G` should jump to the last
loaded row immediately; if more data exists, it should also start the next-page
command. A failed next-page fetch must keep the already loaded rows visible.

### Phase Use

- Phase 3 must define the first concrete list paging state for Issues.
- Phase 4 should reuse the same list state for Pull Requests and keep PR
  preview loading separate from page loading.
- Phase 5 must add log-specific incremental loading because Actions logs are
  much larger and noisier than issue or PR metadata.

### Future API Extension Points

`internal/github.Service` currently exposes `Issues`, `PullRequests`, and
`CheckSummary` methods without caller-controlled paging. It also exposes
`WorkflowRuns` with a limit and `WorkflowRunLogs` with explicit log options,
but neither method returns a resumable page token. Future work should extend
these read-only boundaries rather than changing rendering code to call `gh`
directly or adding another Actions service path. A future shape could be:

```go
type ListPageRequest struct {
	Repo   string
	State  string
	Limit  int
	Cursor string
}

type ListPage[T any] struct {
	Items      []T
	NextCursor string
	FetchedAt  time.Time
}
```

The names and concrete item types can differ, but the app should only receive
normalized data and opaque cursors.

## Sorting

### Policy

Sort functions should be pure, deterministic, and covered by small tests in the
domain package for the relevant view. Each sortable item should project into a
small common schema:

```go
type ReadingSortInput struct {
	ID        string
	Number    int
	Title     string
	State     string
	UpdatedAt time.Time
	Attention ReadingAttentionState
	Check     workbench.CheckState
	Draft     bool
}
```

`workbench.WorkItem` compatibility matters because the workbench already has
issue, pull request, review, check, and `UpdatedAt` data. Workbench-backed rows
can populate the schema from `WorkItem.Issue`, `WorkItem.PullRequest`, and
`WorkItem.Checks`; dedicated views can populate it from their own item types.

Default sort should be `updatedAt` descending, with stable tie breakers:

1. Attention-needed bucket.
2. Lifecycle/status bucket.
3. Item number descending.
4. Stable item ID.

Attention-needed is view-specific but should normalize to shared buckets:

- `needs_viewer`: assigned issue, review requested from viewer, or failed run
  that needs attention.
- `waiting_on_others`: viewer-authored PR waiting on review or pending run.
- `blocked_or_failing`: failing checks, failed workflow run, or blocked status.
- `normal`: no special attention state.

The UI should add a local sort switcher in the active list. `s` should cycle
through the available sort modes, and contextual help should name the active
mode. Sort changes should reorder loaded memory immediately and start a
background refresh only when the remote query shape changes.

### Phase Use

- Phase 3 needs `updatedAt`, `state`, and an optional issue attention bucket.
- Phase 4 needs the complete PR attention model: review requested,
  waiting-on-review, draft, and failed checks.
- Phase 5 needs status/conclusion sorting for workflow runs and failed log
  attention.

## Text Search Within Active View

### Policy

Search should be local and non-regex for the first implementation. It should
fold case by default with `strings.ToLower` or an equivalent Unicode-safe helper
chosen during implementation. Search should match a stable field set per view:

- Issues: number, title, state, labels, assignees, milestone, and body excerpt.
- Pull Requests: number, title, state, author, head branch, base branch, review
  state, requested reviewers, linked issues, and body excerpt.
- Actions: run number or database ID, workflow name, branch, event, status,
  conclusion, actor, job name, and visible log excerpt.

`/` should enter a focused search input for the active view. While the input is
open, typing updates the local filtered list without starting network work.
`enter` accepts the query, `esc` closes the input without changing the previous
accepted query, and an empty accepted query clears the filter. Clearing search
should restore the prior selection when the selected item is still present.

Search must never block movement. If search is active while a next page arrives,
the new rows are merged into memory and then filtered locally.

### Phase Use

- Phase 3 should establish the prompt behavior with Issues.
- Phase 4 should reuse the prompt and extend the PR field set.
- Phase 5 should allow searching workflow runs and loaded log excerpts without
  requiring full log downloads.

## Truncation And Explicit Expand

### Policy

List rows should never include large body or log text. Preview panes may show
bounded excerpts:

- Issue and PR bodies: at most 80 rendered lines or 16 KiB of source text,
  whichever is reached first.
- Workflow run summaries: at most 80 rendered lines.
- Logs: at most 200 lines or 64 KiB per loaded segment.

Truncation should preserve whole lines when possible and append a visible
footer such as `-- truncated; press e to expand --`. `e` should expand the
focused preview. For issue and PR bodies, expansion may show the full loaded
body when it is already in memory. For logs, expansion should load the next log
segment or a focused failed-step segment through `internal/github`; full log
loading must remain explicit.

Expanded state should belong to the focused object identity and should reset
when the focused object, repository, accepted search query, lifecycle filter, or
sort mode changes. Refresh should also return the preview to the truncated
state unless the implementation can prove the expanded content still belongs to
the same object version.

### Phase Use

- Phase 3 needs issue body truncation and the first expand/collapse behavior.
- Phase 4 should apply the same rules to PR bodies and review/check context.
- Phase 5 must treat log expansion as a separate paged data load, not just a
  render toggle.

## Empty / Loading / Partial-Error / Stale-Data States

### Policy

All reading views should render consistent state rows or banners. Use existing
semantic styles from `internal/app/styles.go` before adding new style tokens:

- Empty:
  - Icon: `-`
  - Text: `No <items> match this view`
  - Style: `Muted`
  - Fallback: keep filters visible; clearing search or filters should be
    obvious in help.
- Initial loading:
  - Icon: `...`
  - Text: `Loading <items>...`
  - Style: `Muted`
  - Fallback: movement in other panes remains available.
- Loading more:
  - Icon: `...`
  - Text: `Loading more <items>...`
  - Style: `Muted`
  - Fallback: keep loaded rows selectable.
- Partial error:
  - Icon: `!`
  - Text: `Some <items> could not be loaded. Press r to retry.`
  - Style: `Warning` or `Danger` by severity.
  - Fallback: preserve loaded rows, keep the next cursor if retry is possible,
    and retry the failed page or preview only.
- Stale data:
  - Icon: `~`
  - Text: `Showing cached <items>; refresh in progress.`
  - Style: `Warning`
  - Fallback: show cached rows while a refresh request is active and discard
    stale responses by request ID.

Partial errors in dedicated views should be view state, not synthetic
`workbench.WorkItem` rows. The existing workbench error-item pattern can remain
for the workbench pipeline, but issue, PR, and Actions list views should keep
errors out of the normal sortable item list.

### Phase Use

- Phase 3 should introduce the shared renderer or local renderer contract with
  Issues.
- Phase 4 should prove that PR-specific preview and check errors can use the
  same state vocabulary.
- Phase 5 should add page-level and log-segment partial errors, because Actions
  data has the highest chance of partial failure.

## Movement Responsiveness

### Policy

Cursor movement must update synchronously from local state. Network fetches,
preview loading, and log loading should run as Bubble Tea commands and return
messages with enough identity to discard stale results.

Responsibilities should stay separated:

- Local cache: loaded rows, selected index, accepted search query, sort mode,
  expanded preview identity, and stale flags.
- Network fetch: only starts from explicit commands or near-end incremental
  triggers, and never runs inside rendering helpers.
- Preview state: keyed by object identity and request ID, so old results cannot
  replace the preview after the user moves.

If data is still loading, `j`, `k`, `g`, `G`, pane focus keys, and search input
editing should continue to work against the loaded rows. Starting a fetch should
not clamp the cursor to zero unless the underlying item set has genuinely
changed and the previous item identity cannot be found.

### Phase Use

- Phase 3 should prove the behavior for issue list movement and issue preview
  loading.
- Phase 4 should keep PR previews, checks, and linked issue enrichment from
  blocking PR list movement.
- Phase 5 should keep run-list movement independent from log loading, because
  logs are the slowest and largest data source.

## Implementation Notes

- Do not introduce a direct GitHub API client for these P2 items. Use `gh` and
  `gh api` behind `internal/github`.
- Do not add mutating actions as part of the reading experience work.
- Prefer small, pure packages for filter and sort logic once at least one
  dedicated view exists.
- Keep style additions conservative. If the state table above cannot be
  expressed through the existing `Styles` fields, add semantic tokens in the
  same spirit as ADR 0008 during the view implementation issue.
- Use small tests for sorting, filtering, truncation, request identity, stale
  response discard, and state rendering. Use medium tests with large fake
  issue, PR, Actions, and log payloads or fake command boundaries. Reserve
  large tests for authenticated `gh` and other real external systems.

## Sub-Issue Plan

### 1. Add paged Issues list state

- Dependency: phase 3, Issues view.
- Minimum scope: define the first Issues list state with first-page loading,
  in-memory chunk storage, an opaque next cursor, stale flags, and near-end
  incremental fetch triggers.
- Acceptance estimate: focusing the Issues view loads a bounded first page,
  moving near the end requests another page, already loaded rows stay visible
  after a page error, and no app rendering code calls `gh` directly.

### 2. Add deterministic reading sort modes

- Dependency: phase 3 for Issues first; phase 4 for PR attention fields; phase
  5 for Actions status fields.
- Minimum scope: add pure sort inputs and sort functions for `updatedAt`,
  status, and attention-needed order, plus an active-list `s` sort cycle.
- Acceptance estimate: loaded rows reorder locally without refetching, stable
  tie breakers make output deterministic, and workbench-compatible issue/PR
  data can populate the sort input.

### 3. Add active-view text search

- Dependency: phase 3, Issues view, then reused by phase 4 and phase 5.
- Minimum scope: add `/` search input, non-regex folded-case filtering,
  accepted-query state, cancellation, clearing, and selection restoration for
  the active list.
- Acceptance estimate: typing filters loaded rows immediately, `esc` preserves
  the previous accepted query, an empty accepted query clears search, and
  movement remains responsive while the prompt is open.

### 4. Add preview truncation and explicit body expansion

- Dependency: phase 3 Issues preview and phase 4 PR preview.
- Minimum scope: apply body excerpt caps, render a truncation footer, add `e`
  expand/collapse for the focused issue or PR body, and reset expansion when
  focus or query context changes.
- Acceptance estimate: large bodies do not overflow preview rendering,
  expansion is explicit, and stale expanded content cannot remain attached to a
  different focused object.

### 5. Add consistent reading view state rendering

- Dependency: phase 3, Issues view.
- Minimum scope: render empty, initial loading, loading more, partial-error,
  and stale-data states using the shared copy and existing semantic styles.
- Acceptance estimate: each state has deterministic view tests, partial errors
  preserve loaded rows, and retry behavior is exposed through `r`.

### 6. Keep list movement independent from fetch and preview loading

- Dependency: phase 3 for Issues, phase 4 for PRs, and phase 5 for Actions.
- Minimum scope: add request identity and stale-result discard for list page
  loads and previews, and ensure movement keys work while commands are running.
- Acceptance estimate: tests can move focus after starting a load, stale
  results do not overwrite the new focus, and cursor position is preserved by
  stable item identity where possible.

### 7. Extend Actions run and log reading behavior

- Dependency: phase 5, existing Actions / logs view (`modeActions` and
  `actionsState`).
- Minimum scope: add resumable chunks to the existing workflow run list, add
  status and updated-time sorting, and evolve the existing explicit log fetch
  into segmented expansion without replacing its request identity or stale
  result handling.
- Acceptance estimate: the existing Actions view keeps its current navigation
  and on-demand log behavior, run lists paginate like issue/PR lists, logs are
  truncated by line and byte caps, failed log loading is a partial preview
  error, and full log retrieval never happens implicitly during cursor
  movement.
