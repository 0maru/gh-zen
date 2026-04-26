# 0005: Package as a GitHub CLI Extension

- Status: Accepted
- Date: 2026-04-26

## Context

`gh-zen` should be usable as a GitHub CLI extension. GitHub CLI extensions have
packaging and execution constraints that should guide the repository layout
before the application grows.

The GitHub CLI extension contract is repository-oriented:

- The repository name must start with `gh-`.
- The command name is the repository name without the `gh-` prefix, so this
  repository is invoked as `gh zen`.
- An extension cannot override a core `gh` command. If a name conflicts with a
  core command, users must run it through `gh extension exec`.
- Arguments after `gh zen` are forwarded to the extension executable.
- A script extension must provide an executable file at the repository root with
  the same name as the repository.
- A precompiled extension can instead provide release assets with platform
  suffixes recognized by `gh`.
- For local development installs with `gh extension install .`, `gh` links to an
  executable file named like the repository at the repository root. For this
  project, that file is `gh-zen`.

For Go extensions, GitHub's manual creation example builds from repository root
with `go build`, and the `cli/gh-extension-precompile` action supports standard
Go extension builds while allowing custom `go build` options when needed.

## Decision

Treat `gh-zen` as a precompiled Go GitHub CLI extension.

Keep a root `main.go` with `package main` as the thin process entrypoint. The
root entrypoint should stay small and delegate application behavior to internal
packages, such as `internal/app`.

The root `main.go` is not required because GitHub CLI directly reads Go source.
GitHub CLI runs an executable, not the source tree. However, keeping the Go
entrypoint at repository root is the default-compatible path for:

- `go build`.
- `go build -o gh-zen .`.
- Local extension development with `gh extension install .` after building the
  root `gh-zen` binary.
- Future release automation with `cli/gh-extension-precompile`.

Do not move the main package into `cmd/gh-zen` unless there is a concrete reason
to accept extra release and local-install configuration. If the entrypoint ever
moves, the release workflow and local development instructions must be updated
at the same time.

Do not commit built extension binaries. The root `gh-zen` binary should be
ignored by Git and treated as a local build artifact.

The development loop for extension behavior is:

```sh
go build -o gh-zen .
gh extension install .
gh zen
```

If the extension is already installed from the same local checkout, rebuilding
`gh-zen` is enough for the next `gh zen` run to use the new binary.

Release distribution should use precompiled release assets. Asset names must use
the `gh-zen-{os}-{arch}` shape, with `.exe` for Windows. Release automation
should prefer `cli/gh-extension-precompile@v2` unless the project needs custom
build behavior that the action cannot express.

## Consequences

Positive:

- Users get the expected GitHub CLI command shape: `gh zen`.
- The root Go entrypoint keeps local `go build` and extension release automation
  simple.
- `scripts/build.sh` gives local development and future release automation a
  shared build entrypoint. `make build` is the preferred manual shortcut for
  that script.
- Application code can still live under `internal/` and remain testable.
- The project does not need a root shell wrapper or checked-in binary.

Tradeoffs:

- The repository root keeps a Go entrypoint file instead of moving all
  executables under `cmd/`.
- Local extension installs require a root `gh-zen` binary to exist before
  execution succeeds.
- Release automation needs to produce platform-specific assets before remote
  users can install a binary release.

## Implementation Notes

The root `main.go` should remain boring:

- Parse only process-level concerns that must happen before the application
  model starts.
- Construct and run the Bubble Tea program.
- Print fatal startup errors to stderr.
- Delegate UI and GitHub workflow behavior to internal packages.

The root binary can be built with:

```sh
./scripts/build.sh
```

The script writes `gh-zen` at the repository root by default. It can also build
to a specific output path when release or packaging work needs that shape:

```sh
./scripts/build.sh dist/gh-zen-darwin-arm64
```

For manual development, use the Makefile shortcut:

```sh
make build
```

Before publishing the first release, add a release workflow that uses
`cli/gh-extension-precompile@v2` with `go_version_file: go.mod`, then verify a
tagged release can be installed by `gh extension install 0maru/gh-zen`.

For local development, verify the extension path with:

```sh
./scripts/build.sh
gh extension install .
gh zen
```

The equivalent Makefile path is:

```sh
make install-extension
gh zen
```

If `gh` reports that the extension already exists, remove it with
`gh extension remove zen` or use the appropriate `gh extension install` flag
before reinstalling from the local checkout.

## Alternatives Considered

- Put the executable package under `cmd/gh-zen`: idiomatic for multi-command Go
  repositories, but it adds build configuration without a current benefit.
- Use a root shell wrapper that runs `go run`: convenient for local development,
  but it requires users to have Go installed and does not match the precompiled
  extension goal.
- Commit the built `gh-zen` binary: simple for local install, but it bloats the
  repository and conflicts with GitHub's guidance for precompiled extensions.
- Start as an interpreted extension: simpler packaging at first, but it would
  make the TUI depend on users having the right interpreter and toolchain.

## Maintenance Notes

This ADR should be revisited when:

- The repository stops being distributed as a GitHub CLI extension.
- The executable package moves out of repository root.
- Release automation switches away from `cli/gh-extension-precompile`.
- The extension name changes from `zen`.

If the packaging strategy changes materially, add a new ADR and mark this one as
`Superseded` instead of rewriting this decision in place.
