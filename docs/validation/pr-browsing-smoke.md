# PR Browsing Smoke

This is a manual smoke test for the first-class pull request browser. It is
large-gate equivalent and is not required for the normal local check.

## Setup

```sh
gh extension install .
gh zen
```

Use a repository that has at least:

- one open pull request
- one draft pull request
- one merged pull request
- one pull request with requested review
- one pull request with failing checks

## Checks

1. Press `]` from a work item that has a pull request.
2. Confirm the pull request view opens and preselects that PR.
3. Confirm rows show PR number, state, draft status, title, author, review,
   checks, and updated date.
4. Press `j` and `k` and confirm preview content follows focus without blocking
   navigation.
5. Press `/`, enter a text query, and confirm the list filters locally.
6. Press `f` and toggle state, author, review, checks, and draft filters.
7. Press `y`, `Y`, `H`, and `o` and confirm URL, PR number, head ref, and
   browser-open actions work.
8. Press `[` and confirm the workbench selection is preserved.
