# 0003: Use Yazi-Inspired Object Navigation and Previews

- Status: Accepted
- Date: 2026-04-26

## Context

`gh-zen` is intended to become a focused terminal workflow for GitHub data. The
project already uses Bubble Tea and related Charmbracelet libraries as the TUI
foundation, but the product interaction model is still early.

Yazi is a useful reference because it makes terminal file navigation feel fast,
continuous, and inspectable:

- Moving the cursor updates adjacent context immediately.
- Preview work happens asynchronously and does not freeze navigation.
- Stale preview work can be discarded when the user's focus changes.
- Common actions are close to the selected item and can be applied to multiple
  selected entries.
- The interface favors direct keyboard movement over modal page transitions.

These qualities should be translated to GitHub objects rather than copied as a
file-manager UI.

## Decision

Use a Yazi-inspired navigation and preview model for GitHub objects.

The default layout should grow toward a three-region object browser:

- Left: context, scope, repository, query, or saved view navigation.
- Middle: the current list of GitHub objects, such as pull requests, issues,
  notifications, workflow runs, releases, or search results.
- Right: a preview of the focused object, such as Markdown content, review
  comments, checks, diff summaries, workflow failures, or notification thread
  context.

Cursor movement should be cheap and continuous. Focusing an item should start
loading its preview without requiring an explicit open action. Navigation must
remain responsive while preview data is loading.

Preview loading should be asynchronous and discardable:

- A preview request belongs to the object that was focused when the request was
  started.
- If focus moves before the request returns, the stale result must not replace
  the preview for the newly focused object.
- Cached preview data may be reused when it is still valid for the current
  object and view.
- Loading, empty, error, and partial states should be visible without blocking
  the rest of the UI.

Keyboard interaction should prioritize direct movement and local action:

- `j` and `k` move through the current list.
- `h` and `l` move outward and inward through context when that hierarchy
  exists.
- `/` filters or searches within the current view.
- `g` and `G` jump to the start and end of a list.
- `space` toggles selection when batch actions are available.
- `?` opens contextual help.

The exact key map may evolve, but new shortcuts should preserve this direct,
Vim-like navigation style unless a better local convention emerges.

Batch actions should be first-class where GitHub semantics make them safe and
useful. Examples include marking notifications as read, copying URLs, adding a
label, assigning users, opening selected objects in the browser, or applying a
common issue or pull request transition.

Long-form text entry should continue to follow ADR 0002. Comments, issue bodies,
pull request descriptions, and review text should use the configured external
editor path for comfortable Unicode and IME-heavy input.

The first concrete product slice should be a current-repository pull request
browser:

- Left: saved pull request scopes, such as open, assigned, review requested, or
  failed checks.
- Middle: pull request list.
- Right: pull request body, checks, review state, and diff summary.

This slice should prove the core interaction loop before the same model is
expanded to issues, notifications, workflow runs, releases, or cross-repository
search.

## Consequences

Positive:

- Users can inspect GitHub work without repeatedly opening browser pages.
- The UI has a clear interaction identity instead of becoming a set of unrelated
  command screens.
- Asynchronous previews keep navigation responsive even when GitHub API calls
  are slow.
- The layout creates a natural home for Glamour-rendered Markdown, check status,
  diff summaries, and review context.
- The same object-browser model can scale across multiple GitHub workflows.

Tradeoffs:

- The model needs careful request identity, caching, and stale-result handling.
- Three-region layouts require explicit behavior for narrow terminals.
- Preview rendering can become noisy if each object type invents unrelated
  presentation rules.
- Batch actions need clear confirmation and error handling when they mutate
  remote GitHub state.

## Implementation Notes

Keep the first implementation narrow and testable:

- Model focused object identity separately from loaded preview data.
- Represent preview states explicitly, including idle, loading, loaded, empty,
  and error.
- Route GitHub IO through commands or a narrow service boundary rather than view
  rendering.
- Prefer existing Bubbles components for lists, spinners, help, and text input
  before building custom widgets.
- Use Glamour for Markdown previews once real GitHub content is displayed.
- Add update-loop tests for focus changes, stale preview responses, selection,
  and key bindings before expanding the surface area.

For narrow terminals, prefer preserving the focused list and preview over
showing every region at once. The left context region can collapse before the
main list or preview loses essential information.

## Alternatives Considered

- Browser launcher only: easy to build, but it would not create a distinct
  terminal workflow or reduce context switching.
- Command palette only: useful for direct actions, but weaker for browsing and
  comparing many GitHub objects.
- File-manager clone: visually familiar, but GitHub objects need richer preview
  states, remote loading, batch mutation, and Markdown-aware rendering.
- Single-page detail screens: simpler layout, but they make scanning and rapid
  comparison slower.

## Maintenance Notes

This ADR should be revisited when:

- The application adopts a different primary navigation model.
- Preview loading becomes synchronous or blocks movement.
- Batch actions become the dominant workflow over browsing and previewing.
- The app introduces a second object layout that conflicts with this model.

If the interaction model changes materially, add a new ADR and mark this one as
`Superseded` instead of rewriting this decision in place.
