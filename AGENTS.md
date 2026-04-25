# Repository Instructions

## Communication

- Respond to the user in Japanese unless they ask for another language.
- Keep repository documents, code comments, commit messages, PR titles, and PR
  descriptions in English.

## Workflow

- Inspect existing files and project conventions before editing.
- Keep changes scoped to the requested task.
- Prefer existing Go, Bubble Tea, and Charmbracelet patterns over introducing
  new abstractions.
- Run relevant validation after changes, and note any checks that could not be
  run.

## Validation

- Use Lefthook v2.1.1 or newer.
- Install local Git hooks with `lefthook install` after cloning.
- Run the fast local gate with `lefthook run pre-commit` or
  `./scripts/test-small.sh`.
- Run the normal local gate with `lefthook run check` or `./scripts/check.sh`.
- Run the push gate with `lefthook run pre-push`.
- Keep large tests opt-in with `GH_ZEN_LARGE_TESTS=1 ./scripts/test-large.sh`.
- Use small tests for deterministic model, view, and pure logic coverage.
- Use medium tests for local integration with temporary files, fixtures, and
  fake command boundaries.
- Use large tests only for real external systems such as GitHub API,
  authenticated `gh`, browser automation, or long-running end-to-end workflows.

## GitHub

- Use English for commit messages.
- Use English for pull request titles and descriptions.
- Avoid destructive Git operations unless the user explicitly asks for them.
