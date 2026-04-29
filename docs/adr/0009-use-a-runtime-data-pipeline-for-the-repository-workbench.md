# 0009: Use a Runtime Data Pipeline for the Repository Workbench

- Status: Accepted
- Date: 2026-04-29

## Context

ADR 0006 chose the repository workbench as the primary navigation model and
intentionally started with deterministic fake data. Issues #12 through #17 then
added the pieces needed for real data:

- Local Git discovery for worktrees, branches, dirty state, detached state, and
  missing or prunable worktrees.
- Work item assembly from local repository state.
- A GitHub service boundary backed by `gh`.
- Pull request, issue, and check linking for work items.
- Runtime workbench filters from resolved configuration.

Those pieces are useful only when the application entrypoint composes them. The
current TUI still starts with fake repositories and fake work items, so the next
implementation slice needs a durable direction for replacing fake startup data
without pushing Git or GitHub calls into rendering code.

## Decision

Use a runtime data pipeline to populate the repository workbench.

The application startup path should resolve a runtime context, construct data
services, load local work items first, enrich them with GitHub data when
available, and then pass the resulting state into the Bubble Tea model.

The pipeline order is:

1. Load and validate configuration.
2. Resolve the startup repository from ADR 0007.
3. Resolve the local repository path when the startup repository maps to a
   checkout or current Git repository.
4. Build work items from local Git state through the local repository service.
5. Load pull requests through the GitHub service and link them to branch-backed
   work items.
6. Link issues from PR metadata, issue lists, and conservative branch-name
   heuristics.
7. Load check summaries for linked pull requests.
8. Apply workbench filters to the resolved work items.
9. Keep diagnostics from failed discovery or enrichment visible in the
   workbench instead of crashing the TUI.

Local data is the base layer because it is what the user can act on
immediately. GitHub data is an enrichment layer. If GitHub auth, network, or a
single PR check lookup fails, the workbench should still show local work items
and add non-fatal error state.

The Bubble Tea model should receive resolved runtime data and service handles
through constructors or commands. Rendering code must not call Git, `gh`, the
GitHub API, or the filesystem directly.

Fake repositories and fake work items should remain available for deterministic
tests and explicit demo or fallback paths, but they must not be the default when
the application can resolve real repository state.

Refresh should rerun the same pipeline for the selected repository rather than
calling only a GitHub summary endpoint. Refresh should update the work item list
and preview context, preserving selection by stable work item ID when possible.

## Consequences

Positive:

- The product becomes useful with real local worktrees, branches, pull requests,
  issues, and checks.
- Local work remains visible even when GitHub enrichment is unavailable.
- Data loading stays testable because Git and GitHub access remain behind
  service boundaries.
- The fake workbench remains useful for deterministic UI tests without
  controlling production behavior.

Tradeoffs:

- Startup and refresh need explicit loading, error, and partial-result states.
- The pipeline may issue several Git and `gh` commands, so refresh should be
  asynchronous and cancelable or discard stale responses.
- Local and remote state can disagree. The UI must represent uncertainty rather
  than silently choosing one source as authoritative.
- Real data loading introduces medium and large validation paths beyond the
  small deterministic model tests.

## Implementation Notes

Prefer a small runtime composition layer over putting orchestration into
`main.go` or rendering helpers. A future shape could be:

```go
type RuntimeServices struct {
	Local  workbench.LocalDiscovery
	GitHub github.Service
}

type WorkbenchLoader interface {
	Load(ctx context.Context, repo workbench.RepoRef) WorkbenchLoadResult
}
```

The exact types may differ, but the implementation should preserve these
boundaries:

- `main.go` stays a thin process entrypoint as described in ADR 0005.
- `internal/config` owns configuration and startup repository resolution.
- `internal/localrepo` owns Git command execution and parseable Git output.
- `internal/github` owns `gh` command execution and GitHub-specific error
  classification.
- `internal/workbench` owns work item assembly and linking logic.
- `internal/app` owns Bubble Tea state, commands, rendering, and selection.

Initial real-data implementation should cover one selected repository before
adding multi-repository discovery from configured roots.

When no repository can be resolved, the TUI may start with an empty workbench or
an explicit demo mode, but it should not silently show fake data as if it were
live repository state.

Error handling should follow these rules:

- Invalid configuration remains fatal before the TUI starts.
- Missing current Git repository is non-fatal when no stronger startup
  repository is configured.
- Local discovery failure becomes a local-discovery work item for that
  repository.
- GitHub discovery failure becomes a non-fatal workbench item or status message.
- Per-PR check lookup failure does not stop remaining PR check lookups.

Testing should follow ADR 0004:

- Small tests cover pipeline ordering, partial-result behavior, selection
  preservation, and fake service composition.
- Medium tests cover temporary Git repositories and fake `gh` command runners.
- Large tests are opt-in and cover authenticated `gh` behavior against real
  repositories.

## Alternatives Considered

- Keep fake data until all real integrations are complete: simple, but it hides
  integration problems and delays the point where the app is useful.
- Load GitHub data first: useful for remote-only workflows, but it makes local
  worktrees and dirty state secondary despite being the primary workbench base.
- Let the Bubble Tea model call Git and `gh` directly: fewer orchestration
  types, but rendering and update logic would become hard to test and reason
  about.
- Fail startup when any discovery step fails: clearer for scripts, but poor for
  a TUI whose job is to show partial state and guide the user to the problem.

## Maintenance Notes

This ADR should be revisited when:

- The workbench stops being local-first.
- The project replaces `gh` with direct GitHub API clients for normal runtime
  data.
- Multi-repository loading changes the pipeline order.
- Refresh becomes event-driven or background-synchronized instead of
  user-triggered.

If the runtime data strategy changes materially, add a new ADR and mark this one
as `Superseded` instead of rewriting this decision in place.
