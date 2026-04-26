# gh-zen

`gh-zen` is a GitHub CLI extension for focused terminal GitHub workflows.

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

Install this checkout as a local GitHub CLI extension:

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
