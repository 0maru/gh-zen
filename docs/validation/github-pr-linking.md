# GitHub Pull Request Linking Validation

This plan covers real GitHub behavior for linking repository work items to pull
requests. It is intentionally manual until large tests are enabled for
authenticated `gh` behavior.

## Prerequisites

- `gh auth status` succeeds for an account that can read the target repository.
- The target repository has at least one local branch with an open pull request.
- The target repository has at least one local branch without a pull request.

## Plan

1. Run the local work item discovery for the repository.
2. Load pull requests with the GitHub service for the same `owner/repo`.
3. Verify that a work item whose branch matches `headRefName` has PR number,
   URL, state, review state, and an unknown check placeholder.
4. Verify that a branch without a matching pull request remains visible and
   renders `no PR`.
5. Repeat with an expired or missing GitHub authentication state and confirm the
   app receives a non-fatal pull request discovery error item.
