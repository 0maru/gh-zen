# 0002: Keep UI Copy in English and Support Unicode Input

- Status: Accepted
- Date: 2026-04-25

## Context

`gh-zen` should provide a stable terminal workflow for GitHub data while keeping
the product surface small. Application UI copy can stay in English, but user
input and GitHub content must support Japanese and other Unicode text.

Japanese input in a TUI has two distinct risks:

- Inline composition can be fragile when an IME emits non-ASCII text through
  terminal key events.
- Long-form issue, PR, or comment text is more comfortable in a user's editor
  than in a small terminal composer.

OpenAI Codex CLI was reviewed as a reference point at commit
`706490ab1b3f79ba807581b35aeeff6222e04cac`. Its TUI keeps UI copy in English,
supports non-ASCII/IME input without holding the first non-ASCII character in
paste detection, and opens an external editor only from `$VISUAL` or `$EDITOR`.
It does not silently fall back to `vim`.

## Decision

Keep application UI copy in English, including labels, help text, menu items,
shortcut hints, and error messages.

Treat Japanese and other Unicode text as first-class user input and content:

- Preserve UTF-8 text for input fields, search queries, issue titles, comments,
  and rendered GitHub content.
- Do not use byte length or rune count for terminal display width, cursor
  movement, truncation, or wrapping.
- Prefer Bubbles input components and Charmbracelet display-width-aware helpers
  before implementing custom text editing behavior.
- Keep input normalization separate from stored or submitted text. If search
  needs normalization, normalize only the search query/index path and preserve
  the original text for display and GitHub API calls.

Support an external editor path for long-form or IME-sensitive input:

- Use `ctrl+g` as the shortcut to edit the current draft in an external editor.
- Resolve the editor from `$VISUAL`, then `$EDITOR`.
- Do not fall back to `vim`, `vi`, or any other editor when both variables are
  unset. Show an actionable error instead.
- Seed a temporary Markdown file with the current draft, run the editor while
  the TUI terminal is released, then replace the draft with the saved file
  contents after the editor exits successfully.
- Preserve the existing draft if the editor command cannot be resolved, fails to
  start, or exits unsuccessfully.
- Trim trailing whitespace from the saved editor contents before applying it.

In Go/Bubble Tea, external editor execution should use Bubble Tea's blocking
exec path, such as `tea.ExecProcess`, so stdin/stdout and the alternate screen
are released and restored by the TUI runtime.

## Consequences

Positive:

- English UI copy keeps the application surface simpler and easier to test.
- Japanese issue titles, comments, and search text remain supported without
  turning the whole UI into a localization project.
- Users with complex IME workflows can use their configured editor instead of
  relying only on inline terminal input.
- Avoiding a built-in `vim` fallback prevents surprising users who have not
  chosen an editor for this application.

Tradeoffs:

- New users without `$VISUAL` or `$EDITOR` must configure an editor before using
  external editing.
- The inline composer still needs Unicode display-width and cursor tests.
- External editor behavior depends on the user's shell environment and terminal.

## Implementation Notes

The first implementation should stay narrow:

- Add a small editor resolver that parses `$VISUAL` before `$EDITOR`.
- Add a draft editor command that writes and reads a temporary `.md` file.
- Add a Bubble Tea command wrapper that launches the editor with terminal
  control released.
- Add focused tests for editor resolution, missing editor errors, failed editor
  preservation, successful UTF-8 round trips, and trailing whitespace trimming.

Inline composer tests should cover at least:

- Japanese text.
- Mixed Japanese and ASCII text.
- Emoji and multi-codepoint grapheme clusters.
- Narrow terminal widths.
- Paste and IME-like non-ASCII input sequences.

## Alternatives Considered

- Localized Japanese UI: useful later, but too much surface area before the core
  workflows are stable.
- Built-in fallback to `vim`: convenient for some users, but surprising and
  inconsistent with the user's configured terminal environment.
- Inline-only input: simpler to build at first, but weaker for long Japanese
  text and less resilient to terminal IME edge cases.

## Maintenance Notes

This ADR should be revisited when:

- The project adds full localization.
- External editor behavior changes away from `$VISUAL`/`$EDITOR`.
- The composer starts implementing custom cursor or wrapping logic.
- Search adds Japanese-specific normalization rules.

If the editor policy changes, add a new ADR and mark this one as `Superseded`
instead of silently changing the input contract.
