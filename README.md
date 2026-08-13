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
- Read-only workbench actions: open a linked pull request or issue, copy the
  best available GitHub URL, copy the local worktree path, and refresh the
  workbench data.
- GitHub Actions mode: browse workflow runs for the selected repository,
  preview run and job details, fetch failed logs on demand, and open or copy
  run context.

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
- To install from source with `make install-extension`, Go 1.26.1 or newer and
  `make` on `PATH`.
- A local GitHub checkout for the repository you want to inspect.
- An `origin` remote that points to `github.com` so `gh-zen` can resolve
  `owner/repo`.
- Optional on Linux: `xdg-open` for URL-opening actions, and `wl-copy` or
  `xclip` for clipboard actions.

`gh-zen` keeps local Git data visible when GitHub enrichment is unavailable. In
that case the workbench may show a non-fatal discovery error item for missing
authentication, network failures, or unavailable GitHub data.

## Usage

Start from inside a GitHub checkout:

```sh
cd ~/workspaces/github.com/0maru/gh-zen
gh zen
```

The startup repository is resolved from `GH_ZEN_REPO`, then `startup.repo` in
configuration, and finally the current Git checkout. Once the workbench opens:

1. Pick a repository or view in the left pane.
2. Move through work items in the center pane.
3. Read the combined local and GitHub context in the preview pane.
4. Use read-only actions to open URLs, copy context, or refresh data.
5. Press `a` to inspect GitHub Actions runs for the selected repository, then
   press `w` to return to the workbench.

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
| `Waiting on review` | Pull requests authored by the authenticated viewer and waiting on review from another reviewer. |
| `Failed checks` | Work items whose linked pull request has failing checks. |

In Actions mode, the pane layout becomes:

| Pane | Purpose |
| --- | --- |
| `Repositories` | Select the repository whose workflow runs should be loaded. |
| `Runs` | Recent workflow runs for the selected repository, with in-memory filters. |
| `Preview` | Run metadata, jobs, failure summary, and explicitly loaded failed logs. |

## Key Bindings

The TUI currently uses these built-in defaults. Key remapping through
configuration is not wired into the TUI yet.

Common keys:

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
| `refresh` | `r` | Reload data for the current mode. |
| `quit` | `q`, `esc`, `ctrl+c` | Exit the TUI. |

Workbench mode keys:

| Action ID | Default keys | Behavior |
| --- | --- | --- |
| `show_actions` | `a` | Open the GitHub Actions view for the selected repository. |
| `open_pr` | `p` | Open the linked pull request URL. |
| `open_issue` | `i` | Open the linked issue URL. |
| `copy_url` | `y` | Copy the linked pull request URL, or the linked issue URL when no PR URL exists. |
| `copy_worktree_path` | `Y` | Copy the local worktree path. |

Actions mode keys:

| Action ID | Default keys | Behavior |
| --- | --- | --- |
| `show_workbench` | `w` | Return to the repository workbench. |
| `open_workflow_run` | `o` | Open the selected workflow run URL. |
| `copy_url` | `y` | Copy the selected workflow run URL. |
| `copy_workflow_run_id` | `Y` | Copy the selected workflow run ID. |
| `fetch_workflow_run_logs` | `L` | Fetch failed logs for the selected workflow run. |

Actions mode filter keys:

| Action ID | Default keys | Behavior |
| --- | --- | --- |
| `filter_status` | `s` | Cycle the status filter. |
| `filter_conclusion` | `c` | Cycle the conclusion filter. |
| `filter_branch` | `b` | Cycle the branch filter. |
| `filter_workflow` | `n` | Cycle the workflow name filter. |
| `filter_event` | `e` | Cycle the event filter. |
| `filter_actor` | `u` | Cycle the actor filter. |
| `clear_filters` | `x` | Clear all Actions filters. |

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

[workbench.filter]
worktree = "/home/alice/workspaces/github.com/0maru/gh-zen*"
branch_pattern = "feat/*"
pull_request = "any"
local_status = "any"
```

`repos.roots` is used to discover local checkouts and expands `~`.
`workbench.filter.worktree` narrows visible work items by absolute worktree path
glob. The worktree filter does not expand `~`.

## Current Limitations

- First-class issue browsing is not implemented. Issues appear only as linked
  context on work items.
- Full pull request detail browsing is not implemented. The workbench shows
  linked PR summary, review state, and check state rather than a complete PR
  page.
- Actions mode is read-only and focused on recent workflow runs, run preview,
  filters, opening run URLs, copying run context, and explicit failed-log
  fetches. It does not manage workflow runs.
- Mutating GitHub or Git operations are intentionally out of scope for the
  current action set.

## Further Reading

- [Architecture Decision Records](docs/adr/README.md)
- [0006: Use Repository Workbench as the Primary Navigation Model](docs/adr/0006-use-repository-workbench-as-the-primary-navigation-model.md)
- [0007: Use Layered Configuration With Terminal Profiles](docs/adr/0007-use-layered-configuration-with-terminal-profiles.md)
- [0009: Use a Runtime Data Pipeline for the Repository Workbench](docs/adr/0009-use-a-runtime-data-pipeline-for-the-repository-workbench.md)
- [GitHub Actions Smoke Validation](docs/validation/github-actions-smoke.md)
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
