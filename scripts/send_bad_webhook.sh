#!/usr/bin/env bash
# Sends a fake Shopify orders/create webhook we expect Klaviyo to reject: it has
# no customer email, so the event should be stored as `received`, forwarded, and
# come back `failed` with Klaviyo's error in last_error. Mirrors send_webhook.sh
# (valid HMAC) but uses a current created_at so the worker's age gate forwards it
# instead of skipping it. Requires SHOPIFY_WEBHOOK_SECRET; PORT defaults to 8080.
set -euo pipefail

: "${SHOPIFY_WEBHOOK_SECRET:?SHOPIFY_WEBHOOK_SECRET must be set}"

CREATED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
PAYLOAD=$(printf '{"id":9999000001,"name":"#9001","total_price":"49.99","currency":"USD","created_at":"%s","line_items":[{"title":"Mystery Box","quantity":1,"price":"49.99"}]}' "$CREATED_AT")

SIG=$(printf '%s' "$PAYLOAD" | openssl dgst -sha256 -hmac "$SHOPIFY_WEBHOOK_SECRET" -binary | base64)

curl -s -w '\n%{http_code}\n' -X POST "http://localhost:${PORT:-8080}/webhook/shopify/orders" \
  -H "Content-Type: application/json" \
  -H "X-Shopify-Hmac-SHA256: $SIG" \
  -d "$PAYLOAD"
