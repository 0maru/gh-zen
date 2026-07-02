# gh-zen

[![CI](https://github.com/0maru/gh-zen/actions/workflows/ci.yml/badge.svg)](https://github.com/0maru/gh-zen/actions/workflows/ci.yml)

`gh-zen` is a GitHub CLI extension for terminal-first GitHub browsing.

The product direction is to keep the daily loop of local branches, worktrees,
pull requests, review state, checks, and linked issues visible from the
terminal. It is not a full browser replacement. The current focus is a
repository workbench that starts from local Git state and enriches it with
GitHub data when authenticated `gh` access is available.

## Current Capabilities

- Repository workbench: a Bubble Tea TUI with repository navigation, repo-scoped
  work items, built-in workbench views, and a preview pane.
- Local worktree and branch discovery: local worktrees are loaded from
  `git worktree list --porcelain`, branches are discovered from Git refs, and
  local status is summarized as clean, dirty, detached, missing, or unknown.
- GitHub enrichment: pull requests, linked issues, review state, and check
  summaries are linked to local work items through `gh`.
- Read-only actions: open a linked pull request or issue, copy the best
  available GitHub URL, copy the local worktree path, and refresh the workbench
  data.

## Installation

Install the latest published release with GitHub CLI:

```sh
gh extension install 0maru/gh-zen
```

Until the first release is published, install this repository as a local
extension:

```sh
git clone https://github.com/0maru/gh-zen
cd gh-zen
make install-extension
```

Run it with:

```sh
gh zen
```

## Prerequisites

- `git` on `PATH`.
- GitHub CLI (`gh`) on `PATH`; run `gh auth status` to confirm authentication
  when you want GitHub enrichment.
- A local GitHub checkout for the repository you want to inspect.
- An `origin` remote that points to `github.com` so `gh-zen` can resolve
  `owner/repo`.
- Optional for Linux clipboard actions: `wl-copy` or `xclip`.

`gh-zen` keeps local Git data visible when GitHub enrichment is unavailable. In
that case the workbench may show a non-fatal discovery error item for missing
authentication, network failures, or unavailable GitHub data.

## Usage

Start from inside a GitHub checkout:

```sh
cd ~/workspaces/github.com/0maru/gh-zen
gh zen
```

The startup repository is resolved from configuration, `GH_ZEN_REPO`, or the
current Git checkout. Once the workbench opens:

1. Pick a repository or view in the left pane.
2. Move through work items in the center pane.
3. Read the combined local and GitHub context in the preview pane.
4. Use read-only actions to open URLs, copy context, or refresh data.

## Panes and Views

| Pane | Purpose |
| --- | --- |
| `Repositories` | Repositories discovered from configured roots plus workbench views. |
| `Work Items` | Repo-scoped items assembled from worktrees, branches, pull requests, issues, checks, and local status. |
| `Review` | Preview for the selected repository or work item. |

The left pane also includes these views:

| View | Contents |
| --- | --- |
| `Active worktrees` | Work items that have a local worktree. |
| `Needs my review` | Pull requests where the authenticated viewer or one of their teams is requested for review. |
| `Waiting on review` | Pull requests waiting on review from another reviewer. |
| `Failed checks` | Work items whose linked pull request has failing checks. |

## Key Bindings

The TUI currently uses these built-in defaults. Key remapping through
configuration is not wired into the TUI yet.

| Action ID | Default keys | Behavior |
| --- | --- | --- |
| `move_down` | `j`, `down` | Move down in a selectable pane. |
| `move_up` | `k`, `up` | Move up in a selectable pane. |
| `jump_top` | `g` | Jump to the first item. |
| `jump_bottom` | `G` | Jump to the last item. |
| `focus_next_pane` | `l`, `tab` | Move focus to the next pane. |
| `focus_previous_pane` | `h`, `shift+tab` | Move focus to the previous pane. |
| `focus_pane_1` | `1` | Focus the first visible pane. |
| `focus_pane_2` | `2` | Focus the second visible pane. |
| `focus_pane_3` | `3` | Focus the third visible pane. |
| `toggle_help` | `?` | Toggle full contextual help. |
| `refresh` | `r` | Reload workbench data for the selected repository. |
| `open_pr` | `p` | Open the linked pull request URL. |
| `open_issue` | `i` | Open the linked issue URL. |
| `copy_url` | `y` | Copy the linked pull request URL, or the linked issue URL when no PR URL exists. |
| `copy_worktree_path` | `Y` | Copy the local worktree path. |
| `quit` | `q`, `esc`, `ctrl+c` | Exit the TUI. |

## Configuration

Configuration is layered from built-in defaults, global config, project config,
and an optional terminal profile selected by `GH_ZEN_TERMINAL`.

Default config paths:

```text
~/.config/gh-zen/config.toml
./.gh-zen.toml
~/.config/gh-zen/terminals/<terminal-id>.toml
```

Minimal example:

```toml
[startup]
repo = "0maru/gh-zen"
view = "workbench"

[repos]
roots = ["~/workspaces/github.com/0maru"]

[repos.repositories."0maru/gh-zen"]
default_branch = "main"
worktree_root = "~/workspaces/github.com/0maru"

[workbench.filter]
worktree = "/home/alice/workspaces/github.com/0maru/gh-zen*"
branch_pattern = "feat/*"
pull_request = "any"
local_status = "any"
```

`repos.roots` is used to discover local checkouts. The per-repository
`worktree_root` key is accepted by the configuration model for repository-scoped
settings, and `workbench.filter.worktree` narrows visible work items by absolute
worktree path glob. The worktree filter does not expand `~`.

## Current Limitations

- First-class issue browsing is not implemented. Issues appear only as linked
  context on work items.
- Full pull request detail browsing is not implemented. The workbench shows
  linked PR summary, review state, and check state rather than a complete PR
  page.
- GitHub Actions browsing is not implemented. Check summaries can be shown for
  linked pull requests, but workflow runs and logs are not browsable.
- Mutating GitHub or Git operations are intentionally out of scope for the
  current action set.

## Further Reading

- [Architecture Decision Records](docs/adr/README.md)
- [0006: Use Repository Workbench as the Primary Navigation Model](docs/adr/0006-use-repository-workbench-as-the-primary-navigation-model.md)
- [0007: Use Layered Configuration With Terminal Profiles](docs/adr/0007-use-layered-configuration-with-terminal-profiles.md)
- [0009: Use a Runtime Data Pipeline for the Repository Workbench](docs/adr/0009-use-a-runtime-data-pipeline-for-the-repository-workbench.md)
- [GitHub Pull Request Linking Validation](docs/validation/github-pr-linking.md)
- [Live Data Smoke Validation](docs/validation/live-data-smoke.md)

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
