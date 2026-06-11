#!/usr/bin/env bash
# Sends a fake Shopify orders/create webhook to the local server with a valid
# HMAC signature. Requires SHOPIFY_WEBHOOK_SECRET to be set (and matched by the
# running server). PORT defaults to 8080.
set -euo pipefail

: "${SHOPIFY_WEBHOOK_SECRET:?SHOPIFY_WEBHOOK_SECRET must be set}"

PAYLOAD='{"id":5678901234,"name":"#1042","email":"jane@example.com","total_price":"129.99","currency":"USD","created_at":"2026-06-11T07:00:00Z","line_items":[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]}'

SIG=$(printf '%s' "$PAYLOAD" | openssl dgst -sha256 -hmac "$SHOPIFY_WEBHOOK_SECRET" -binary | base64)

curl -s -w '\n%{http_code}\n' -X POST "http://localhost:${PORT:-8080}/webhook/shopify/orders" \
  -H "Content-Type: application/json" \
  -H "X-Shopify-Hmac-SHA256: $SIG" \
  -d "$PAYLOAD"
