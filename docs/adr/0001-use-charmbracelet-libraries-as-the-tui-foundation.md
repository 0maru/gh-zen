# 0001: Use Charmbracelet Libraries as the TUI Foundation

- Status: Accepted
- Date: 2026-04-25

## Context

`gh-zen` is starting as a Go terminal application. The current `go.mod` already
contains the core shape of the TUI stack:

- `github.com/charmbracelet/bubbletea`: terminal program runtime and update loop.
- `github.com/charmbracelet/lipgloss`: terminal styling and layout.
- `github.com/charmbracelet/bubbles`: reusable TUI components.
- `github.com/charmbracelet/glamour`: Markdown rendering for GitHub-oriented
  content.
- `github.com/cli/go-gh/v2`: Go integration with GitHub CLI behavior and
  authentication context.

The initial implementation in `main.go` uses Bubble Tea and Lip Gloss directly.
The rest of the module graph is still early and should be allowed to change as
the product surface becomes clearer.

## Decision

Use the Charmbracelet ecosystem as the default foundation for the TUI:

- Bubble Tea owns the application loop, message handling, and commands.
- Lip Gloss owns terminal styling and layout.
- Bubbles should be preferred for common interactive controls before building
  custom widgets.
- Glamour should be used when the app needs to render Markdown-rich GitHub data.
- GitHub access should stay behind a narrow application boundary instead of
  being called directly from view rendering or key handling.

This keeps the app idiomatic for Go, testable at the update/model boundary, and
aligned with libraries already present in the module graph.

## Consequences

Positive:

- The UI can grow from a small model without introducing a custom terminal
  framework.
- Bubble Tea keeps state transitions explicit and testable.
- Lip Gloss and Bubbles reduce one-off terminal layout code.
- Glamour gives the project a natural path for rendering issue, PR, or
  Markdown content.

Tradeoffs:

- The project inherits Charmbracelet API and terminal behavior changes.
- Terminal layout and display-width edge cases still need explicit testing.
- The TUI model can become coupled to GitHub IO unless commands and data access
  are kept behind boundaries.
- Some dependencies may remain indirect until concrete features use them.

## Alternatives Considered

- Plain `fmt` output and manual terminal handling: simpler at first, but likely
  to accumulate bespoke state and layout code quickly.
- A different TUI framework such as `tview`: useful for widget-heavy apps, but it
  does not match the current Bubble Tea model already present in the codebase.
- A web UI: unnecessary for the current command-line workflow and would add a
  separate runtime surface.

## Maintenance Notes

This ADR should be revisited when:

- Bubble Tea, Lip Gloss, Bubbles, or Glamour is removed or replaced.
- A second TUI framework is introduced.
- GitHub access starts leaking directly into view rendering.
- The project has enough UI surface that shared layout, key binding, or command
  patterns need their own ADR.

If this decision stops being current, add a new ADR and mark this one as
`Superseded` instead of rewriting the history of why the stack was chosen.
