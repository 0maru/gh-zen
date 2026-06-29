# Repository Root Discovery Validation

This manual smoke test verifies that configured repository roots populate the
Repository Workbench with multiple local GitHub checkouts.

## Prerequisites

- `gh auth status` succeeds for an account that can read the repositories.
- At least two GitHub repositories are cloned below one configured root.
- Each checkout has an `origin` remote on `github.com`.

## Plan

1. Run `make build`.
2. Create or update a local gh-zen config so `repos.roots` includes the parent
   directory that contains at least two checkouts.
3. Optionally add one missing path to `repos.roots` to verify non-fatal
   diagnostics.
4. Run `./gh-zen`.
5. Confirm the repository pane lists both repositories discovered from the
   configured root.
6. Focus the repository pane and move between repositories. Confirm the preview
   pane shows the local path, default branch, remotes, active worktree count,
   open PR count, open issue count, and failing check count for the selected
   repository.
7. Confirm local work items remain visible even when the missing root produces a
   `repository discovery error` work item.
8. Press `r` to refresh and confirm the selected repository and selected work
   item are preserved when they still exist.
