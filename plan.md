# Plan: shopify-klaviyo-relay

## Context

A Go HTTP service that receives Shopify `orders/create` webhook events, stores them in PostgreSQL, and forwards them to the Klaviyo Events API as "Placed Order" events. Simulates the kind of integration a loyalty platform builds between e-commerce and marketing tools. One day of focused work. No real Shopify store needed.

---

## Development Order

One vertical at a time — each slice is fully working (tests green, manually verifiable) before the next begins. No scaffolding ahead. Dev tooling (`scripts/`, `Makefile`, `.env.example`, `README.md`) is built up incrementally alongside whichever vertical needs it.

### Vertical 1 — Webhook ingestion

Everything needed to receive an order and store it:

- `config/config.go` — env loading (DB + SHOPIFY_WEBHOOK_SECRET only; Klaviyo not needed yet)
- `store/schema.sql` + DB connection in `store/store.go`
- `api/middleware.go` — HMAC verification
- `api/handler.go` — webhook handler only (`POST /webhook/shopify/orders`)
- `main.go` — wire config, DB, single route
- Tests: HMAC unit tests, handler integration test (POST → row in DB with status `received`)
- Manual check: `make send-webhook` → row visible in DB

### Vertical 2 — Klaviyo forwarding

Everything needed to forward stored events and track outcomes (single attempt, no retry yet):

- `config/config.go` — add Klaviyo config vars
- `worker/worker.go` — poll loop, single Klaviyo HTTP call, update status to `succeeded` or `failed`
- `store/store.go` — add update queries (status, last_attempted_at, last_error)
- Tests: Klaviyo mock with `httptest.NewServer`, success/failure status transitions
- Manual check: worker picks up `received` row, status moves to `succeeded` (or `failed` with a bad key)

### Vertical 3 — Frontend & events API

Everything needed to observe what's happening:

- `api/handler.go` — add `GET /api/events` (JSON) and `GET /` (HTML)
- `store/store.go` — add list query
- HTML template + vanilla JS polling
- `store/seed.sql` — demo rows for all statuses
- Tests: `GET /api/events` returns correct shape
- Manual check: open browser, see live status updates

### Vertical 4 — Retry & backoff

Everything needed to handle failures gracefully:

- `config/config.go` — add worker retry config vars (`MAX_RETRIES`, `RETRY_INITIAL_INTERVAL`, `RETRY_MULTIPLIER`, `MAX_EVENT_AGE`, `WORKER_POLL_INTERVAL`)
- `worker/worker.go` — add backoff delay computation, expiry check, `retrying` status, retry loop
- `store/store.go` — add `retry_count` update query
- Tests: backoff delay computation, expiry check, max retries exhaustion, `expired` status transition
- Manual check: worker retries a failing event with increasing delays, eventually marks `failed` or `expired`

---

## Architecture

### Request flow

1. `POST /webhook/shopify/orders` — read raw body, verify HMAC, parse JSON, write row with status `received`, return 200 immediately
2. Background worker goroutine polls DB every 5s for `received`/`retrying` events; backoff and expiry decisions made in Go
3. Forward to Klaviyo Events API as "Placed Order"
4. Success → update status to `succeeded`
5. Failure → `retry_count++`, `last_attempted_at = NOW()`, status `retrying`; if `ordered_at` older than `MAX_EVENT_AGE` → `expired`
6. After `MAX_RETRIES` failures → status `failed`, store `last_error`
7. `GET /` — server-rendered HTML event log; vanilla JS polls `GET /api/events` for live updates

### Why PostgreSQL as queue (not Redis/RabbitMQ)

- Events are already persisted before returning 200 — durability is free
- Retry state (`retry_count`, `last_attempted_at`) lives naturally as columns; backoff config stays in the app
- Frontend reads status directly from DB with no extra plumbing
- Minimal dependencies rule; Redis is appropriate when throughput demands it

### Why 200 immediately

Shopify enforces a 5s total timeout and retries 8 times over 4 hours on failure. Returning 200 on receipt ACKs Shopify's delivery — all subsequent retries are our concern, not Shopify's.

---

## Code Conventions

- Standard library first — add a dependency only when stdlib genuinely can't do the job
- Always pass `context.Context` as the first argument to every I/O function
- Wrap errors: `fmt.Errorf("doing thing: %w", err)`
- Structured logging with `log/slog` (JSON output) at every meaningful step
- No comments unless the WHY is non-obvious

---

## Project Structure

```bash
shopify-klaviyo-relay/
├── main.go               -- wiring: config, DB, worker, server
├── api/
│   ├── handler.go        -- webhook handler, frontend handler, /api/events
│   └── middleware.go     -- HMAC-SHA256 verification
├── store/
│   ├── store.go          -- DB operations (insert, update status, list)
│   ├── schema.sql        -- table definition
│   ├── seed.sql          -- demo rows covering all statuses
│   └── queries.sql       -- sqlc annotated queries
├── worker/
│   └── worker.go         -- background retry loop + Klaviyo forwarding
├── config/
│   └── config.go         -- env var loading, validated at startup
├── scripts/
│   └── send_webhook.sh   -- generate fake Shopify payload + valid HMAC, POST to local server
├── sqlc.yaml
├── Dockerfile            -- multi-stage: build in golang:1.26, run in alpine
├── docker-compose.yml    -- app + postgres services; no local Go required
├── .env.example          -- all keys with safe defaults; KLAVIYO_API_KEY and SHOPIFY_WEBHOOK_SECRET blank
├── Makefile              -- fmt, lint, test, sqlc generate, docker up, seed
└── README.md
```

---

## Database Schema

```sql
CREATE TYPE event_status AS ENUM ('received', 'retrying', 'succeeded', 'failed', 'expired');

CREATE TABLE webhook_events (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    shopify_order_id BIGINT       NOT NULL,
    order_name       TEXT         NOT NULL,       -- e.g. "#1001"
    customer_email   TEXT         NOT NULL,
    total_price      NUMERIC      NOT NULL,
    currency         TEXT         NOT NULL,
    line_items       JSONB        NOT NULL,        -- variable array, kept as JSONB
    ordered_at       TIMESTAMPTZ  NOT NULL,        -- order.created_at from Shopify
    status           event_status NOT NULL DEFAULT 'received',
    retry_count      INT          NOT NULL DEFAULT 0,
    last_attempted_at TIMESTAMPTZ,                 -- null = never attempted
    last_error       TEXT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

Statuses: `received` → `retrying` → `succeeded` / `failed` / `expired`

---

## HMAC Verification (`api/middleware.go`)

Shopify signs the raw request body with the app's client secret.

```bash
Header:  X-Shopify-Hmac-SHA256  (base64-encoded HMAC-SHA256)
Key:     SHOPIFY_WEBHOOK_SECRET env var
Data:    raw request body (read before json.Decode)
Compare: hmac.Equal — timing-safe
```

Implementation note: read `r.Body` into `[]byte` once, pass to both the HMAC check and `json.Unmarshal`. Reject with 401 on mismatch.

---

## Klaviyo Mapping

```bash
POST https://a.klaviyo.com/api/events/
Authorization: Klaviyo-API-Key <KLAVIYO_API_KEY>
revision: 2023-10-15

{
  "data": {
    "type": "event",
    "attributes": {
      "metric": { "data": { "type": "metric", "attributes": { "name": "Placed Order" } } },
      "profile": { "data": { "type": "profile", "attributes": { "email": order.email } } },
      "value": order.total_price (float),
      "properties": {
        "order_id":   order.id,
        "order_name": order.name,
        "line_items": order.line_items,
        "currency":   order.currency
      },
      "time": order.created_at
    }
  }
}
```

---

## Retry Worker (`worker/worker.go`)

- Goroutine started in `main.go`, receives `context.Context` for clean shutdown
- Ticker: configurable poll interval (default 5s)
- Query: `SELECT ... WHERE status IN ('received', 'retrying') FOR UPDATE SKIP LOCKED`
- All backoff and expiry decisions made in Go, not SQL — DB stores facts, app owns policy

**Per-event decision logic (in Go):**

```go
// 1. Expiry check — anchor on ordered_at (customer's perspective)
if time.Since(event.OrderedAt) > cfg.MaxEventAge {
    // mark expired, skip
}

// 2. Backoff check
delay := cfg.RetryInitialInterval * time.Duration(math.Pow(cfg.RetryMultiplier, float64(event.RetryCount)))
if event.LastAttemptedAt != nil && time.Since(*event.LastAttemptedAt) < delay {
    // not ready yet, skip
}
```

- On success → status `succeeded`
- On failure, retries remaining → `retry_count++`, `last_attempted_at = NOW()`, status `retrying`
- On failure, max retries exhausted → status `failed`, store `last_error`
- On age exceeded → status `expired`, store `last_error = "expired: order older than max age"`

---

## Config (`config/config.go`)

```go
# Database
DB_HOST                  -- required (e.g. localhost or db in docker-compose)
DB_PORT                  -- default 5432
DB_NAME                  -- required
DB_USER                  -- required
DB_PASSWORD              -- required

# External services
KLAVIYO_API_KEY          -- required
SHOPIFY_WEBHOOK_SECRET   -- required

# Server
PORT                     -- default 8080

# Worker
MAX_RETRIES              -- default 5
RETRY_INITIAL_INTERVAL   -- default 1s
RETRY_MULTIPLIER         -- default 2.0
MAX_EVENT_AGE            -- default 24h
WORKER_POLL_INTERVAL     -- default 5s
```

`config.go` composes DB vars into a DSN (`postgres://user:pass@host:port/name`). Fail fast at startup if any required var is missing.

---

## Frontend

- `GET /` — server-rendered HTML table (Go `html/template`)
- `GET /api/events` — JSON list for polling
- Vanilla JS `setInterval` every 3s to refresh the table
- Columns: Order ID, Status, Retries, Last Error, Created At
- No React, no build step

---

## Testing

### TDD Workflow

Strict red-green cycle, one case at a time:

1. Write a single test case — run it — watch it fail
2. Write the minimum implementation to make that one case pass
3. Ask for input on what to test next (suggestions offered, decision is yours)
4. Repeat — add the next case to the table only after the previous one is green

Table-driven tests (`t.Run`) are used, but the table is built incrementally — never written all at once upfront.

### Unit Tests (first)

Small, fast, no DB or network. Key targets:

- HMAC verification (valid signature, invalid signature, missing header)
- Retry backoff logic (delay computation per `retry_count`)
- Expiry check (`ordered_at` vs `MAX_EVENT_AGE`)
- Status transition rules
- Klaviyo request builder (correct JSON shape)
- Klaviyo HTTP call mocked with `httptest.NewServer`

### Integration Tests (once structure exists)

Exercise the full path from HTTP request to DB state:

- `POST /webhook/shopify/orders` → row inserted with status `received`
- Worker tick → status transitions correctly for success/failure/expiry cases
- `GET /api/events` → returns correct JSON reflecting DB state

Run with `go test -race`; require a real PostgreSQL instance (via docker-compose in CI).

---

## Dev Tooling

### `scripts/send_webhook.sh`

Generates a realistic fake Shopify order payload and POSTs it to the local server with a valid HMAC signature. Uses `openssl` (available on macOS by default):

```bash
#!/usr/bin/env bash
PAYLOAD=$(cat <<'EOF'
{"id":5678901234,"name":"#1042","email":"jane@example.com","total_price":"129.99",
 "currency":"USD","created_at":"2026-06-11T07:00:00Z",
 "line_items":[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]}
EOF
)
SIG=$(printf '%s' "$PAYLOAD" | openssl dgst -sha256 -hmac "${SHOPIFY_WEBHOOK_SECRET}" -binary | base64)
curl -s -X POST http://localhost:${PORT:-8080}/webhook/shopify/orders \
  -H "Content-Type: application/json" \
  -H "X-Shopify-Hmac-SHA256: $SIG" \
  -d "$PAYLOAD"
```

### `store/seed.sql`

Inserts demo rows covering all statuses so the frontend looks alive on first load:

```sql
INSERT INTO webhook_events (shopify_order_id, order_name, customer_email, total_price, currency, line_items, ordered_at, status, retry_count) VALUES
  (1001, '#1001', 'alice@example.com',  89.99, 'USD', '[{"title":"Sneakers","quantity":1,"price":"89.99"}]',   NOW() - interval '1 hour',   'succeeded', 0),
  (1002, '#1002', 'bob@example.com',   149.00, 'USD', '[{"title":"Jacket","quantity":1,"price":"149.00"}]',   NOW() - interval '3 hours',  'failed',    5),
  (1003, '#1003', 'carol@example.com',  49.50, 'USD', '[{"title":"Book","quantity":2,"price":"24.75"}]',      NOW() - interval '30 hours', 'expired',   2),
  (1004, '#1004', 'dan@example.com',    75.00, 'USD', '[{"title":"Candle Set","quantity":3,"price":"25.00"}]',NOW() - interval '10 minutes','retrying',  2);
```

### Dockerfile

Multi-stage build — no local Go installation needed:

```dockerfile
FROM golang:1.26 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o relay .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/relay .
EXPOSE 8080
CMD ["./relay"]
```

### docker-compose.yml

All values sourced from `.env` — no hardcoded config anywhere:

```yaml
services:
  db:
    image: postgres:16-alpine
    env_file: .env
    environment:
      POSTGRES_DB: ${DB_NAME}
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    ports: ["${DB_PORT}:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER}"]
      interval: 5s
      retries: 5

  app:
    build: .
    ports: ["${PORT}:${PORT}"]
    depends_on:
      db:
        condition: service_healthy
    env_file: .env
```

`.env.example` ships with the repo with all keys present and safe defaults filled in (DB vars, PORT). `KLAVIYO_API_KEY` and `SHOPIFY_WEBHOOK_SECRET` left blank — user must supply them.

### Makefile Targets

```
make fmt            -- go fmt ./...
make lint           -- golangci-lint run
make test           -- go test -race ./...
make generate       -- sqlc generate
make up             -- docker-compose up -d --build
make migrate        -- docker-compose exec db psql -U relay relay < store/schema.sql
make seed-db        -- docker-compose exec db psql -U relay relay < store/seed.sql
make send-webhook   -- bash scripts/send_webhook.sh
```

---

## Prompts Log

AI-assisted decisions recorded in `.claude/prompt-log.md` (per-project hook already configured).

---

## Verification

1. Copy `.env.example` → `.env`, fill in `KLAVIYO_API_KEY` and `SHOPIFY_WEBHOOK_SECRET`
2. `make up` — builds image, starts app + PostgreSQL (no local Go needed)
3. `make migrate` — creates schema inside the DB container
4. `make seed-db` — populates demo rows across all statuses
5. Open `localhost:8080` — event log shows seeded data
6. `make send-webhook` — fires a fake Shopify order with valid HMAC → new `received` row appears, worker picks it up
