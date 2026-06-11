# CLAUDE.md

## Project

`shopify-klaviyo-relay` — Go 1.26 HTTP service. Full spec, architecture, schema, and vertical breakdown in `plan.md`.

## Code Conventions

- Standard library first — add a dependency only when stdlib genuinely can't do it
- Always pass `context.Context` as the first argument to every I/O function
- Wrap errors: `fmt.Errorf("doing thing: %w", err)`
- Structured logging with `log/slog` (JSON output) at every meaningful step
- No comments unless the WHY is non-obvious

## TDD Workflow

1. Write a **single** test case — run it — watch it fail
2. Write the minimum implementation to pass that one case only
3. Ask user what to test next (suggestions welcome, decision is theirs)
4. Add the next table row only after the previous case is green

Use `t.Run()` table-driven tests built incrementally. Always run with `-race`.

## Commit Messages

- Subject: prefix with `fix:`, `bug:`, `feature:`, or `test:` when applicable, then imperative mood, start uppercase, no period, max 50 chars
- Blank line between subject and body
- Body: explain what and why (not how), wrap at 72 chars
- End with a blank line followed by `Co-authored-by:` for all contributors

## Development Approach

One vertical at a time — fully working (tests green, manually verifiable) before starting the next. See `plan.md` for the vertical breakdown.
