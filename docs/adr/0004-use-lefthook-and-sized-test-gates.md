# 0004: Use Lefthook and Sized Test Gates

- Status: Accepted
- Date: 2026-04-26

## Context

`gh-zen` is expected to grow from a small Bubble Tea application into a GitHub
workflow tool with local UI logic, GitHub service boundaries, command execution,
and eventually tests that may depend on real credentials or network access.

The project also needs the same local quality gates to run from several entry
points:

- Git hooks before commit or push.
- Make targets for manual development.
- Manual developer commands.
- Codex project hooks.
- Claude Code project hooks.
- CI jobs once CI is added.

Duplicating `gofmt`, lint, and test commands in each tool would make the checks
drift over time. Running every possible test after every agent edit would also
make the coding loop slow, especially once GitHub integration tests exist.

## Decision

Use Lefthook v2.1.1 or newer as the repository Git hook runner. Use `Makefile`
targets as the local developer command runner. Keep the actual format, lint,
build, and test commands behind scripts in `scripts/`.

The scripts are the stable command boundary:

- `scripts/build.sh` builds the local GitHub CLI extension binary.
- `scripts/fmt.sh` applies formatting.
- `scripts/lint.sh` runs the local lint gate.
- `scripts/test-small.sh` runs the fast deterministic test gate.
- `scripts/test-medium.sh` runs the default local integration test gate.
- `scripts/test-large.sh` runs explicitly enabled external tests.
- `scripts/check.sh` runs the normal local developer check.

Lefthook should call these scripts instead of embedding tool-specific command
logic directly in `lefthook.yml`.

Make targets should also call these scripts instead of becoming a second source
of truth. The Makefile is a discoverable command surface for common local tasks,
not the place where validation or build behavior is defined.

Use sized test gates:

- Small tests are fast, deterministic, and have no network, credentials,
  wall-clock, terminal, browser, or GitHub API dependency. They should cover
  pure logic, Bubble Tea update behavior, deterministic views, and fake service
  boundaries. The command is `go test -short ./...`.
- Medium tests may use local integration surfaces such as temporary
  directories, temporary Git repositories, local command fakes, fixtures, and
  Markdown rendering. They must still avoid real network access and credentials.
  The command is `go test ./...`.
- Large tests may use external systems, real GitHub data, authenticated `gh`
  behavior, browser automation, or long-running end-to-end workflows. They must
  be opt-in through both a build tag and an environment variable. The command is
  `GH_ZEN_LARGE_TESTS=1 go test -tags=large ./...`.

Medium tests that are too slow or environment-sensitive for the small gate
should call `testing.Short()` and skip when `go test -short` is used. Large tests
should use the `large` build tag and also require `GH_ZEN_LARGE_TESTS=1` at
runtime.

Local hook policy:

- `pre-commit` runs formatting, lint, and the small test gate.
- `pre-push` runs the medium test gate.
- `check` runs formatting, lint, and the medium test gate.
- `test-large` is a named Lefthook task and is never wired into normal commit or
  push hooks.

Agent hook policy:

- Codex and Claude Code project hooks should share the same script boundary as
  Lefthook.
- Agent hooks should run only the fast gate after Go-related file edits.
- Agent hooks should not run medium or large tests automatically after every
  edit.
- Agent hooks may apply targeted `gofmt` to edited Go files, then run lint and
  the small test gate.

## Consequences

Positive:

- Local developers, agents, and CI can reuse the same command boundary.
- `make help` gives contributors a low-friction entrypoint without adding a
  language-specific task runner dependency.
- Fast edit loops stay fast because agent hooks stop at the small gate.
- GitHub API and credential-dependent tests have a clear home without slowing
  normal commits.
- The project can add CI later without redesigning the local validation model.
- Lefthook keeps Git hook configuration small and language-agnostic.
- The Makefile keeps local command discovery simple while preserving scripts as
  the implementation boundary.

Tradeoffs:

- Contributors need Lefthook v2.1.1 or newer installed before Git hooks run
  locally.
- Contributors need `make` for the convenience runner, though scripts remain
  directly executable when `make` is unavailable.
- Agent hook behavior depends on Codex and Claude Code project hook support.
- The small, medium, and large boundaries require discipline when new tests are
  added.
- `scripts/check.sh` applies formatting, so it may modify files before running
  the rest of the validation gate.

## Implementation Notes

Keep scripts small and shell-based until the workflow needs richer logic. If a
script grows enough to need structured parsing beyond simple command routing,
move that logic into a small Go helper instead of accumulating fragile shell.

When adding tests:

- Default to small tests for model updates, view rendering, reducers, and pure
  functions.
- Use medium tests for local integration with temporary directories, fixtures,
  and fake command boundaries.
- Use large tests only when real external behavior is the point of the test.
- Do not let large tests run unless `GH_ZEN_LARGE_TESTS=1` is set.

When adding new tools:

- Add or update a script in `scripts/` first.
- Wire Lefthook, Codex, Claude Code, and CI to that script.
- Avoid copying the same command sequence into multiple tool-specific config
  files.

## Alternatives Considered

- Python `pre-commit`: mature and widely used, but it introduces a Python-based
  hook manager into a Go repository and does not remove the need for separate
  agent hook wiring.
- Raw Git hooks: minimal dependencies, but hook scripts would live under
  `.git/hooks` unless manually copied, making them hard to review and share.
- Running `go test ./...` after every agent edit: simple, but likely too slow
  once medium integration tests are added.
- Running large tests by default: too risky because they may require credentials,
  network access, or mutable external state.

## Maintenance Notes

This ADR should be revisited when:

- CI adopts a different test taxonomy.
- Lefthook is removed or replaced.
- Agent hooks become too slow or unreliable for normal development.
- Large tests become safe enough to run automatically in a dedicated CI job.

If the local validation strategy changes materially, add a new ADR and mark this
one as `Superseded` instead of rewriting this decision in place.
