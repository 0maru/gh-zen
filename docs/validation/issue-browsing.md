# Issue Browsing Validation

This plan covers real GitHub behavior for first-class issue browsing in the
terminal. It is intentionally manual because it requires authenticated `gh`,
clipboard access, and browser opening on the reviewer machine.

## Prerequisites

- `gh auth status` succeeds for an account that can read the target repository.
- The target repository has a mix of open and closed issues.
- At least one issue has labels, an assignee, a milestone, and comments.
- At least one pull request links to an issue through GitHub closing references.

## Plan

1. Start `gh zen` from a repository configured for the target `owner/repo`.
2. Select a workbench item that has a linked issue and press `i`.
3. Verify the issue view opens and selects the linked issue.
4. Verify the issue list shows number, title, state, labels, assignees,
   milestone, author, updated time, and comments count where available.
5. Press `s` to cycle open, closed, and all state filters.
6. Press `a`, `b`, and `m` to cycle assignee, label, and milestone filters.
7. Press `/`, enter a query with Unicode text, and confirm matching issues are
   filtered without corrupting the query.
8. Verify the preview shows title, state, labels, assignees, milestone, body
   excerpt, linked pull requests, and comments count when those fields exist.
9. Press `y` to copy the selected issue URL.
10. Press `n` to copy the selected issue number in `#123` form.
11. Press `o` to open the selected issue in the browser.
12. Press `q` or `esc` and verify the original workbench repository, item, and
    pane selection are restored.

## Expected Result

- Issue browsing stays inside the terminal until `o` is explicitly used.
- Filters update the issue list without changing GitHub data.
- Linked pull requests come from existing pull request issue references.
- Comments summary is limited to the comments count.
- Workbench selection is preserved after returning from issue browsing.

## Notes

- ADR 0010 is not added in this change. Track the broader terminal-first GitHub
  browsing product direction in a separate issue if an ADR is still needed.
