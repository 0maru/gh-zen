# Architecture Decision Records

Architecture Decision Records capture durable project decisions. They are not a
general planning scratchpad, and they should stay small enough to remain
trustworthy.

## Current Records

- [0001: Use Charmbracelet Libraries as the TUI Foundation](0001-use-charmbracelet-libraries-as-the-tui-foundation.md)
- [0002: Keep UI Copy in English and Support Unicode Input](0002-keep-ui-copy-in-english-and-support-unicode-input.md)

## Status Values

- `Proposed`: under discussion and not yet the default for new work.
- `Accepted`: the current direction for new work.
- `Superseded`: replaced by a newer ADR.
- `Deprecated`: intentionally retained for history, but no longer guidance.

## Maintenance Rules

- Update an ADR only to clarify the original decision or fix factual mistakes.
- When the decision changes, add a new ADR and mark the old one as
  `Superseded`.
- Delete temporary planning notes once they stop guiding implementation.
- Move historical documents to `docs/archive/` only when they are useful for
  archaeology but should not be part of the active project context.
- Do not depend on an AI-inaccessible location as the source of truth. Active
  documents should be easy for humans and tools to find; obsolete documents
  should be clearly marked or removed from the active index.
