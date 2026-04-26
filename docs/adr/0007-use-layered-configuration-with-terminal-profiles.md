# 0007: Use Layered Configuration With Terminal Profiles

- Status: Accepted
- Date: 2026-04-26

## Context

`gh-zen` should support focused GitHub and local worktree workflows across
multiple terminal sessions. Users may dedicate different terminals to different
repositories, agents, review flows, or worktree filters. The application should
therefore support both general configuration and terminal-specific startup
behavior.

Configuration needs to cover at least:

- Startup repository.
- Startup view.
- Repository roots or worktree include patterns.
- UI preferences.
- Key bindings.
- Workbench filters.

The project should avoid making automatic terminal detection a core dependency.
Terminal names, tab titles, multiplexer panes, and IDE terminals differ across
iTerm, WezTerm, tmux, Zellij, VS Code, and other environments. Explicit
terminal profile selection is more predictable.

## Decision

Use layered TOML configuration with terminal profiles.

Configuration layers are applied in this order, from weakest to strongest:

1. Built-in defaults.
2. Global config.
3. Project config.
4. Terminal profile config.
5. Environment variables.
6. CLI flags and arguments.

Default paths:

```text
~/.config/gh-zen/config.toml
./.gh-zen.toml
~/.config/gh-zen/terminals/<terminal-id>.toml
```

The terminal profile is selected explicitly with `GH_ZEN_TERMINAL`. If
`GH_ZEN_TERMINAL` is unset, terminal-specific config is not loaded. The terminal
ID must be restricted to a safe identifier such as `[A-Za-z0-9_-]+` so it cannot
escape the terminal config directory.

Use `GH_ZEN_REPO` as the environment override for startup repository. Use CLI
flags, such as `--repo`, as the strongest override once CLI parsing exists.

The startup repository resolution order is:

1. `--repo`.
2. `GH_ZEN_REPO`.
3. Terminal profile `startup.repo`.
4. Project config `startup.repo`.
5. Global config `startup.repo`.
6. Current Git repository.
7. Last opened repository, once state persistence exists.

When a current Git repository is available and no stronger startup repo is set,
prefer the current repository over persisted history. This keeps `gh zen`
predictable when launched from inside a checkout or worktree.

Merge rules:

- Scalar values use last-writer-wins.
- Maps are deep-merged.
- Lists are replaced by the stronger layer.
- Key bindings are replaced per action.
- Unknown keys should start as warnings instead of fatal errors.
- Invalid values for known keys should be fatal for the loaded configuration
  layer.

Example global config:

```toml
[startup]
repo = "0maru/gh-zen"
view = "workbench"

[ui]
theme = "default"
preview_width = 0.45
show_icons = true

[keys]
quit = ["q", "ctrl+c"]
help = ["?"]
down = ["j", "down"]
up = ["k", "up"]
refresh = ["r"]
open = ["o"]
copy_url = ["y"]

[repos]
roots = ["~/workspaces/github.com/0maru"]

[worktrees]
include = ["~/workspaces/github.com/0maru/gh-zen*"]
exclude = ["*/tmp/*"]
```

Example project config:

```toml
[startup]
view = "workbench"

[repo]
default_branch = "main"

[workbench]
branch_patterns = ["feat/*", "fix/*", "agent/*"]
```

Example terminal profile:

```toml
[startup]
repo = "0maru/gh-zen"
view = "workbench"

[workbench]
filter = "worktree:agent-a"
```

## Consequences

Positive:

- Users can dedicate terminals to repositories, agents, review workflows, or
  worktree filters without changing global settings.
- Configuration remains predictable because terminal selection is explicit.
- The same merge model can support UI, keymap, repo roots, worktree filters, and
  startup behavior.
- Current-directory behavior stays intuitive for normal CLI use.

Tradeoffs:

- Config merging needs tests for precedence and collection replacement.
- Terminal profile selection requires users to set `GH_ZEN_TERMINAL`.
- List replacement is simpler than list merging, but users must repeat full
  lists when overriding.
- Unknown-key warnings require a warning surface in startup diagnostics.

## Implementation Notes

Keep the first config implementation narrow:

- Define config structs with built-in defaults.
- Load global, project, and terminal profile TOML files.
- Validate terminal IDs before building a terminal profile path.
- Merge layers with explicit tests for each field type.
- Resolve startup repo from config, environment, and current Git repository.
- Defer persistence for last opened repository until the workbench model exists.

Suggested initial paths:

- `internal/config` for config types, loading, validation, and merge behavior.
- `internal/git` or `internal/localrepo` for current repository resolution.

Do not let UI code read files directly. The application model should receive a
resolved runtime configuration.

Testing should follow ADR 0004:

- Small tests cover defaults, merge rules, validation, terminal ID safety, and
  startup repo precedence.
- Medium tests cover temporary project config files and temporary Git
  repositories.
- Large tests are not needed for config loading unless real GitHub behavior is
  involved.

## Alternatives Considered

- Single global config only: simpler, but it cannot support terminal-specific
  agent and review workflows.
- Automatic terminal detection: convenient when it works, but brittle across
  terminals and multiplexers.
- Project config only: useful for repository defaults, but not enough for
  personal UI preferences or per-terminal startup behavior.
- Merge lists by appending: flexible, but surprising for key bindings and
  filters where users usually expect overrides.

## Maintenance Notes

This ADR should be revisited when:

- The config format changes away from TOML.
- Terminal profiles become automatic instead of explicit.
- Startup repository precedence changes.
- State persistence changes the role of last opened repository.

If configuration layering changes materially, add a new ADR and mark this one as
`Superseded` instead of rewriting this decision in place.
