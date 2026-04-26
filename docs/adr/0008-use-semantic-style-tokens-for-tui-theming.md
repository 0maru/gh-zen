# 0008: Use Semantic Style Tokens for TUI Theming

- Status: Accepted
- Date: 2026-04-26

## Context

`gh-zen` should become a dense, scannable terminal UI for repository,
worktree, issue, pull request, check, and review workflows. The current fake
workbench shell is intentionally plain while the navigation model is still
forming, but the target experience should be closer to tools such as `k9s`:
high contrast, quick to scan, and explicit about focus, key bindings, and status
state.

Color and emphasis need to communicate meaning:

- Which pane is active.
- Which row is selected.
- Which keys are currently available.
- Which work items are clean, dirty, blocked, passing, failing, pending, or
  missing.
- Which text is primary information and which text is supporting context.

At the same time, `gh-zen` already has a layered configuration direction in ADR
0007. UI theme settings should eventually be configurable globally, per project,
and per terminal profile. Hard-coding colors directly in render functions would
make that future configuration harder and would spread visual decisions across
the Bubble Tea view code.

## Decision

Use semantic style tokens for TUI rendering.

Rendering code should not choose concrete colors directly. Instead, it should
use named styles that describe meaning and role. Initial tokens should cover at
least:

```go
type Styles struct {
	Title           lipgloss.Style
	Key             lipgloss.Style
	KeyDescription  lipgloss.Style
	Divider         lipgloss.Style
	PaneBorder      lipgloss.Style
	PaneTitle       lipgloss.Style
	PaneTitleActive lipgloss.Style
	Selected        lipgloss.Style
	SelectedMuted   lipgloss.Style
	Muted           lipgloss.Style
	Success         lipgloss.Style
	Warning         lipgloss.Style
	Danger          lipgloss.Style
}
```

The first implementation may keep this type in `internal/app` because only the
workbench shell needs it. Once configuration loading exists, move theme parsing
and default theme construction behind a dedicated package, such as
`internal/theme` or `internal/config`, while keeping the application model
dependent on a resolved `Styles` value.

Start with one built-in dark theme. The theme should be conservative and
terminal-friendly:

- Title: amber or yellow, bold.
- Key chords: cyan or blue, bold.
- Key descriptions: muted foreground.
- Active pane title: cyan or magenta, bold.
- Pane separators and dividers: blue-gray or muted foreground.
- Selected row: bright foreground.
- Inactive selected row marker: muted foreground.
- Success: green.
- Warning and pending: yellow.
- Danger and failing: red.

Style application should be scoped to stable semantic boundaries:

- Header title.
- Header keymap.
- Pane titles.
- Pane separators and horizontal dividers.
- Selection markers.
- Status labels such as checks and local state.
- Muted secondary text.

Avoid styling large arbitrary text blocks at first. Preview content should remain
readable even when ANSI color is disabled or when the terminal theme differs
from the expected dark background.

## Consequences

Positive:

- Visual decisions stay centralized instead of being scattered through render
  functions.
- A future `theme` config can map user colors onto stable semantic roles.
- Tests can focus on semantic rendering behavior and only update output when the
  chosen default theme changes.
- The UI can become more scannable without tying business logic to concrete
  color values.
- Per-terminal theme overrides from ADR 0007 can reuse the same resolved style
  surface.

Tradeoffs:

- Even a small style layer adds indirection to view rendering.
- Golden tests may include ANSI escape sequences once color is enabled.
- Theme configuration needs validation for unknown style keys and invalid color
  values.
- Different terminal color capabilities may require fallback behavior.

## Implementation Notes

Keep the first implementation small:

- Add a `Styles` type and `DefaultStyles()` constructor.
- Store a resolved `Styles` value on the Bubble Tea model.
- Apply styles only to header, keymap, pane titles, separators, selection
  markers, and status labels.
- Keep render helpers responsible for display width after styling. ANSI escape
  sequences must not break alignment.
- Prefer `lipgloss` styles and width helpers over manual ANSI escape sequences.

Testing should follow ADR 0004:

- Small tests cover that styles are applied at expected semantic boundaries.
- Golden tests may include ANSI output when that is the simplest truthful
  representation.
- If ANSI-heavy golden files become hard to review, add a style-disabled test
  mode for structural view tests and keep a smaller number of styled golden
  tests.

Theme configuration should be added after the config loader exists:

```toml
[ui]
theme = "default-dark"

[themes.default-dark]
title = "yellow"
key = "cyan"
muted = "bright-black"
success = "green"
warning = "yellow"
danger = "red"
pane_border = "blue"
```

The exact TOML shape may change, but config should resolve to semantic tokens,
not direct calls from config values to render functions.

Accessibility and compatibility should be handled explicitly:

- Respect `NO_COLOR` or an equivalent config option.
- Add a low-color fallback if 24-bit color assumptions become a problem.
- Avoid relying on color alone for state. Text labels such as `failing`,
  `pending`, and `review requested` should remain visible.
- Revisit light-background support once theme configuration exists.

## Alternatives Considered

- Hard-code colors in render functions: fastest initially, but it would make
  future theme configuration and per-terminal overrides harder.
- Introduce a full theme package immediately: cleaner layering, but premature
  before the config loader and real UI surfaces exist.
- Keep the UI monochrome: simplest and most portable, but weaker for scanning
  dense repository and review state.
- Use color only for status labels: useful, but it misses focus, pane boundary,
  and keymap affordances that make the UI easier to operate.

## Maintenance Notes

This ADR should be revisited when:

- Theme configuration is implemented.
- The style token set becomes too large or too app-specific.
- Golden tests become difficult to maintain because of ANSI output.
- The project adds explicit light, high-contrast, or no-color themes.
- `gh-zen` moves style construction out of `internal/app`.

If the theming model changes materially, add a new ADR and mark this one as
`Superseded` instead of rewriting this decision in place.
