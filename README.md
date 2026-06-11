# shopify-klaviyo-relay

A Go HTTP service that receives Shopify `orders/create` webhook events, stores them in PostgreSQL, and forwards them to the Klaviyo Events API as "Placed Order" events.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose — runs the app and PostgreSQL; no local Go needed to run the service
- [Go 1.26](https://go.dev/dl/) — required for local development (`make test`, `make fmt`, etc.)
- [golangci-lint](https://golangci-lint.run/docs/welcome/install/local/) — required for `make lint` and the pre-commit hook; must be on your `$PATH` after installation
- [lefthook](https://github.com/evilmartians/lefthook/tree/master#install) — git hooks manager; run `lefthook install` once after cloning
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
# 1. Copy env template and fill in your values (SHOPIFY_WEBHOOK_SECRET, KLAVIYO_API_KEY)
cp .env.example .env

# 2. Build and start services (app + PostgreSQL)
make up
```

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
