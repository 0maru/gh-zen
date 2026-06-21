# Issues

## P1 Pull Request Browsing

Anchor: `#p1-pull-request-browsing`

`gh-zen` should let users browse all pull requests for the selected repository
without leaving the terminal.

The first-class PR browser is a parallel view beside the repository workbench:

- It keeps the workbench as the default local-work navigation model.
- It lists open, closed, and merged pull requests sorted by `updatedAt`
  descending.
- It supports filters for state, author, review-requested, waiting-on-review,
  failed checks, draft status, and text search.
- It previews body excerpt, linked issues, review state, requested reviewers,
  latest reviews, checks, and branch refs.
- It supports read-only actions to open the PR, copy the URL, copy the PR
  number, and copy the head ref.

Large/manual smoke validation lives in
[PR Browsing Smoke](validation/pr-browsing-smoke.md).
