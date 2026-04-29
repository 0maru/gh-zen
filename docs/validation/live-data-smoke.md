# Live Data Smoke Validation

This manual smoke test verifies that normal startup uses live repository data
instead of fake workbench fixtures. Real GitHub API validation remains opt-in
because it needs authenticated `gh` state and a suitable repository.

## Prerequisites

- `gh auth status` succeeds for an account that can read the target repository.
- The target repository is cloned locally and has an `origin` remote on
  `github.com`.
- The repository has at least one local branch or worktree.
- Optional: the repository has an open pull request with checks.

## Local Extension Smoke Test

1. From this repository, run `make build`.
2. Install the local extension with `gh extension install . --force`.
3. Change into a real GitHub checkout, for example
   `cd ~/workspaces/github.com/0maru/gh-zen`.
4. Run `gh zen`.
5. Confirm the workbench shows the real checkout repository and local branches
   or worktrees, not the fake fixture repositories.
6. If the checkout has an open pull request, confirm the matching branch shows
   PR, issue, and check data when authenticated GitHub discovery succeeds.
7. Run `gh auth logout` or use an unauthenticated environment, then run `gh zen`
   again and confirm local work items remain visible with a non-fatal GitHub
   discovery error item.

## Opt-In Large Validation

Large tests that touch authenticated GitHub behavior must stay opt-in:

```sh
GH_ZEN_LARGE_TESTS=1 make test-large
```

Do not require large tests for normal `make check` or pre-push validation.
