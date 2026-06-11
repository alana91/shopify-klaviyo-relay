# CLAUDE.md

## Project

`shopify-klaviyo-relay` — Go 1.26 HTTP service. Full spec, architecture, schema, and vertical breakdown in `plan.md`.

## Code Conventions

- Standard library first — add a dependency only when stdlib genuinely can't do it
- Always pass `context.Context` as the first argument to every I/O function
- Wrap errors: `fmt.Errorf("doing thing: %w", err)`
- Structured logging with `log/slog` (JSON output) at every meaningful step
- No comments unless the WHY is non-obvious

## Data & Migrations

- PostgreSQL via the **pgx** driver used through `database/sql` (driver name `"pgx"`); not pgx's native pool API
- **Hand-written SQL** in `store/` — not sqlc
- Migrations: **goose** as a library, files in `store/migrations/*.sql`, embedded and run via `store.Migrate`. Migrations **auto-run at startup** in `main.go` (no separate migrate step). `make db-reset` drops/recreates the local dev DB
- Config: `config.Load` composes everything from env; `DBConfig.DSN()` builds the connection URL

## TDD Workflow

1. Write a **single** test case — **pause for the user to review it** before running or implementing
2. Run it — watch it fail
3. Write the minimum implementation to pass that one case only
4. Ask user what to test next (suggestions welcome, decision is theirs)
5. Add the next table row only after the previous case is green

Use `t.Run()` table-driven tests built incrementally. Always run with `-race`.

## Testing

- Tests hit a **real PostgreSQL** and are **non-negotiable** — never skipped or gated
- `internal/testdb.New` provisions a fresh, uniquely-named database per test, applies migrations, and drops it on cleanup
- `make test` runs locally (Postgres at `localhost`); `make test-docker` runs in our own image on the compose network
- Compare values with `==` / `slices.Equal` / `time.Time.Equal` — **no `reflect.DeepEqual`**

## Commit Messages

- Subject: prefix with `fix:`, `bug:`, `feature:`, or `test:` when applicable, then imperative mood, start uppercase, no period, max 50 chars
- Blank line between subject and body
- Body: explain what and why (not how), wrap at 72 chars
- End with a blank line followed by `Co-authored-by:` for all contributors

## Development Approach

One vertical at a time — fully working (tests green, manually verifiable) before starting the next. See `plan.md` for the vertical breakdown.
