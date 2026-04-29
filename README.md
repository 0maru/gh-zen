# gh-zen

[![CI](https://github.com/0maru/gh-zen/actions/workflows/ci.yml/badge.svg)](https://github.com/0maru/gh-zen/actions/workflows/ci.yml)

`gh-zen` is a GitHub CLI extension for focused terminal GitHub workflows.

## Installation

Install the latest release with GitHub CLI:

```sh
gh extension install 0maru/gh-zen
```

Run it with:

```sh
gh zen
```

## Development

Install local hooks:

```sh
make setup
```

Run the normal local validation gate:

```sh
make check
```

Build the local extension binary:

```sh
make build
```

Install this checkout as a local GitHub CLI extension. `make install-extension`
builds the root `gh-zen` binary first, then installs or updates the local
extension link:

```sh
make install-extension
```

Run it with:

```sh
gh zen
```

`Makefile` targets are convenience entrypoints. The underlying validation and
build commands live in `scripts/` so Lefthook, Codex hooks, Claude Code hooks,
CI, and manual commands can share the same behavior.

## Releases

Pushing a tag that matches `v*`, such as `v0.1.0`, runs the release workflow.
The workflow uses `cli/gh-extension-precompile@v2` to build platform-specific
GitHub CLI extension assets from the root Go package and upload them to the
matching GitHub Release.

Release candidates can use prerelease tags such as `v0.1.0-rc.1`. To test a
specific prerelease, pin the tag explicitly because GitHub CLI resolves an
unpinned install to the latest stable release when one exists:

```sh
gh extension install 0maru/gh-zen --pin v0.1.0-rc.1
```
