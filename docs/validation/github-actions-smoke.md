# GitHub Actions Smoke Validation

This manual smoke test verifies the read-only GitHub Actions browser. Real
GitHub API validation remains opt-in because it needs authenticated `gh` state
and repositories with representative workflow runs.

## Prerequisites

- `gh auth status` succeeds for an account that can read the target repository.
- The target repository is cloned locally and has an `origin` remote on
  `github.com`.
- The repository has recent workflow runs. Ideally include at least one
  successful run, one failed run, and one in-progress or queued run.
- Optional: the failed run has job annotations and failed logs.

## Local Extension Smoke Test

1. From this repository, run `make build`.
2. Install the local extension with `gh extension install . --force`.
3. Change into a real GitHub checkout, for example
   `cd ~/workspaces/github.com/0maru/gh-zen`.
4. Run `gh zen`.
5. Press `a` to switch to GitHub Actions mode.
6. Confirm the middle pane is titled `Runs` and lists workflow runs for the
   selected repository.
7. Move between runs with `j` and `k`. Confirm the preview updates with
   workflow metadata, commit SHA, timing, jobs, and failure summary.
8. Confirm moving focus does not automatically load logs.
9. Press `L` on a failed run. Confirm failed logs appear in the preview only
   after the explicit keypress.
10. Press `o` to open the selected run in the browser.
11. Press `y` to copy the run URL, and `Y` to copy the run ID.
12. Press `r` to refresh workflow runs.
13. Exercise in-memory filters:
    - `s` cycles status.
    - `c` cycles conclusion.
    - `b` cycles branch.
    - `n` cycles workflow name.
    - `e` cycles event.
    - `u` cycles actor.
    - `x` clears all filters.
14. Press `w` to return to the repository workbench.

## Opt-In Large Validation

Large tests that touch authenticated GitHub behavior must stay opt-in:

```sh
GH_ZEN_LARGE_TESTS=1 make test-large
```

Do not require large tests for normal `make check` or pre-push validation.
