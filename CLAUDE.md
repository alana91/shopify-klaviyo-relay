# CLAUDE.md

## Project

`shopify-klaviyo-relay` — Go 1.26 HTTP service. Full spec, architecture, schema, and vertical breakdown in `plan.md`.

## Code Conventions

- Standard library first — add a dependency only when stdlib genuinely can't do it
- Always pass `context.Context` as the first argument to every I/O function
- Wrap errors: `fmt.Errorf("doing thing: %w", err)`
- Structured logging with `log/slog` (JSON output) at every meaningful step
- No comments unless the WHY is non-obvious
- Don't reimplement a vendor's input validation (e.g. Klaviyo's) — send the request and capture/store their error instead

## Data & Migrations

- PostgreSQL via the **pgx** driver used through `database/sql` (driver name `"pgx"`); not pgx's native pool API
- **Hand-written SQL** in `store/` — not sqlc
- Migrations: **goose** as a library, files in `store/migrations/*.sql`, embedded and run via `store.Migrate`. Migrations **auto-run at startup** in `main.go` (no separate migrate step). `make db-reset` drops/recreates the local dev DB
- Config: `config.Load` composes everything from env; `DBConfig.DSN()` builds the connection URL

## HTTP

- Cap untrusted request bodies with `http.MaxBytesReader` before reading them
- Configure an explicit `http.Server` with read/write/header/idle timeouts — not the bare `http.ListenAndServe`
- Wrap the mux in the `Recover` middleware so a handler panic returns 500 instead of dropping the connection
- Shut down gracefully on SIGINT/SIGTERM: drain in-flight HTTP via `srv.Shutdown`, stop the worker, then close the DB. A fatal `ListenAndServe` error still exits non-zero (let the orchestrator restart)
- Return generic error messages to clients (e.g. `"internal error"`); log the details server-side, never leak internals

## TDD Workflow

1. Write a **single** test case — **pause for the user to review it** before running or implementing
2. Run it — watch it fail. If it fails to **compile** because the thing under test doesn't exist yet, add a minimal **stub** (empty/no-op, returning zero values) so it compiles, then run again and watch it fail on the **assertion** — the red step must be a failing test, not a build error
3. Write the minimum implementation to pass that one case only
4. Ask user what to test next (suggestions welcome, decision is theirs)
5. Add the next table row only after the previous case is green

Use `t.Run()` table-driven tests built incrementally. Always run with `-race`.

## Testing

- Tests hit a **real PostgreSQL** and are **non-negotiable** — never skipped or gated
- `internal/testdb.New` provisions a fresh, uniquely-named database per test, applies migrations, and drops it on cleanup
- `make test` runs locally (Postgres at `localhost`); `make test-docker` runs in our own image on the compose network
- Compare values with `==` / `slices.Equal` / `time.Time.Equal` — **no `reflect.DeepEqual`**
- Test organization: variations of the *same* scenario stay in one function as a `t.Run()` table. When a case is materially different from its siblings (different setup, inputs, or what's being asserted), give it its own top-level `TestXxx` function instead of a sibling `t.Run` block

## Commit Messages

- Subject: prefix with `fix:`, `bug:`, `feature:`, or `test:` when applicable, then imperative mood, start uppercase, no period, max 50 chars
- Blank line between subject and body
- Body: explain what and why (not how), wrap at 72 chars
- End the body with a blank line, then `Co-Authored-By: Claude <noreply@anthropic.com>`. The human is the commit *author*, not a co-author — don't list them again as a co-author
- If `.claude/prompt-log.md` is modified, commit it **first** as its own `chore: Update prompt log` commit, then commit the work (split into separate commits as warranted)

## Development Approach

One vertical at a time — fully working (tests green, manually verifiable) before starting the next. See `plan.md` for the vertical breakdown.
