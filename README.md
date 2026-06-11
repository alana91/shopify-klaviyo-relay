# shopify-klaviyo-relay

A Go HTTP service that receives Shopify `orders/create` webhook events, stores them in PostgreSQL, and forwards them to the Klaviyo Events API as "Placed Order" events.

## How it works

The service has two halves that share a PostgreSQL `webhook_events` table:

1. **Ingestion (HTTP).** Shopify POSTs `orders/create` to `/webhook/shopify/orders`. Middleware verifies the `X-Shopify-Hmac-SHA256` signature, the handler parses the order and inserts a row with status `received`, then returns the new event id as JSON.
2. **Forwarding (background worker).** A ticker periodically polls for `received` events, builds a Klaviyo "Placed Order" payload for each, POSTs it to the Klaviyo Events API, and marks the row `succeeded` or `failed`. Events older than `MAX_EVENT_AGE` are skipped.

A small dashboard at `/` polls `GET /api/events` and lists recent events with their status, retry count, and last error.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose — runs the app and PostgreSQL; no local Go needed to run the service
- [Go 1.26](https://go.dev/dl/) — required for local development (`make test`, `make fmt`, etc.)
- [golangci-lint](https://golangci-lint.run/docs/welcome/install/local/) — required for `make lint` and the pre-commit hook; must be on your `$PATH` after installation
- [lefthook](https://github.com/evilmartians/lefthook/tree/master#install) — git hooks manager; installed once via `make setup` after cloning
- A Klaviyo account (see below)

## Klaviyo test account setup

### 1. Create a free account

Go to [klaviyo.com](https://www.klaviyo.com) and sign up. If you already have a production account, use the same email and password but a different company name (e.g. `My Company - Test`). Klaviyo links the accounts to the same login — you can switch between them from the dropdown in the bottom-left corner.

> The signup form asks for a company website. The URL is not verified — any placeholder works (e.g. `https://github.com/yourusername` or `https://example.com`).

### 2. Create a private API key

1. Click your organization name in the **bottom-left corner** → **Settings**
2. In the **General** tab, click **API keys**
3. Click **Create Private API Key**
4. Give it a name (e.g. `shopifyklaviyorelay`)
5. Select **Custom Key**
6. Scroll to **Events** in the scope list and select **Read/Write Access**
7. Click **Create** and **copy the key immediately** — Klaviyo only shows it once

### 3. Add it to your `.env`

```bash
KLAVIYO_API_KEY=pk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

## Local setup

```bash
# 1. One-time per clone: install the git hooks and commit-message template
make setup

# 2. Copy env template and fill in your values (SHOPIFY_WEBHOOK_SECRET, KLAVIYO_API_KEY)
cp .env.example .env

# 3. Build and start services (app + PostgreSQL)
make up
```

`make setup` runs `lefthook install` (pre-commit hooks) and points `commit.template` at `.gitmessage`, so `git commit` opens pre-filled with the message guidelines. Git can't apply a commit template automatically on clone, so each contributor runs this once.

The app applies database migrations automatically on startup, so there is no separate migrate step. To reset the local database to a clean slate (the schema is re-applied on the next start):

```bash
make db-reset
```

## Sending a test webhook

With the service running and `SHOPIFY_WEBHOOK_SECRET` set in your `.env`:

```bash
make send-webhook
```

This fires a fake Shopify `orders/create` payload with a valid HMAC signature. The service verifies the signature, stores the order with status `received`, and returns the new event id as JSON.

## Development

```bash
make help        # list all available targets
make check       # fmt + tidy + lint
make test        # run tests with race detector (needs a local Postgres; run 'make up' first)
make test-docker # run tests in a container on the compose network
```

Tests talk to a real PostgreSQL — each test provisions its own ephemeral database (created and dropped automatically) and applies migrations to it. `make test` reaches Postgres at `localhost`; `make test-docker` runs the suite inside the compose network.

## Configuration

All configuration comes from environment variables (loaded by `config.Load`); copy `.env.example` to `.env` and fill in the blanks. Required variables have no default — the service fails fast at startup if any are missing.

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DB_HOST` | yes | — | Postgres host (`db` inside compose, `localhost` from the host) |
| `DB_PORT` | no | `5432` | Postgres port |
| `DB_NAME` | yes | — | Database name |
| `DB_USER` | yes | — | Database user |
| `DB_PASSWORD` | yes | — | Database password |
| `PORT` | no | `8080` | HTTP listen port |
| `SHOPIFY_WEBHOOK_SECRET` | yes | — | Secret used to verify the webhook HMAC signature |
| `KLAVIYO_API_KEY` | yes | — | Klaviyo private API key |
| `KLAVIYO_BASE_URL` | no | `https://a.klaviyo.com` | Klaviyo API base URL (override for tests/mocks) |
| `WORKER_POLL_INTERVAL` | no | `5s` | How often the worker polls for pending events |
| `MAX_EVENT_AGE` | no | `24h` | Events older than this are skipped by the worker |

`.env.example` also lists `MAX_RETRIES`, `RETRY_INITIAL_INTERVAL`, and `RETRY_MULTIPLIER`. These are reserved for the upcoming retry/backoff work and are **not read yet**.

## Project structure

```
.
├── main.go                  # entrypoint: load config, run migrations, start the worker + HTTP server
├── config/
│   └── config.go            # env-driven configuration; composes the Postgres DSN
├── api/                     # HTTP layer
│   ├── middleware.go        # Shopify HMAC signature verification
│   ├── handler.go           # webhook ingest, events JSON API, and dashboard handlers
│   ├── order.go             # Shopify order parsing and persistence
│   └── index.html           # dashboard, embedded via //go:embed; polls the events API
├── store/                   # persistence layer — hand-written SQL over database/sql + pgx
│   ├── store.go             # Store: InsertEvent, PendingEvents, Mark*, ListEvents, CountEvents
│   ├── migrate.go           # embedded goose migrations, run automatically at startup
│   ├── migrations/          # *.sql schema migrations
│   └── seed.sql             # demo data for the dashboard (make seed-db)
├── worker/                  # background forwarder
│   ├── process.go           # Worker: poll loop (Run), processes pending events, marks succeeded/failed
│   ├── worker.go            # builds the Klaviyo "Placed Order" payload from an event
│   └── klaviyo.go           # Klaviyo Events API HTTP client
├── internal/testdb/
│   └── testdb.go            # test helper: ephemeral per-test Postgres + row seeding
├── scripts/                 # send_webhook.sh / send_bad_webhook.sh — local webhook senders
├── docker-compose.yml       # app + Postgres for local development
├── Dockerfile
├── Makefile                 # dev tasks (run `make help`)
├── lefthook.yml             # pre-commit hooks (fmt, tidy, lint)
└── plan.md                  # full spec, architecture, and vertical breakdown
```

Each top-level package owns one concern (`config`, `api`, `store`, `worker`), wired together in `main.go`. See `plan.md` for the full design and the vertical-by-vertical build plan.
