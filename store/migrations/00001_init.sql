-- +goose Up
CREATE TYPE event_status AS ENUM ('received', 'retrying', 'succeeded', 'failed', 'expired');

CREATE TABLE webhook_events (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    shopify_order_id  BIGINT       NOT NULL,
    order_name        TEXT         NOT NULL,
    customer_email    TEXT         NOT NULL,
    total_price       NUMERIC      NOT NULL,
    currency          TEXT         NOT NULL,
    line_items        JSONB        NOT NULL,
    ordered_at        TIMESTAMPTZ  NOT NULL,
    status            event_status NOT NULL DEFAULT 'received',
    retry_count       INT          NOT NULL DEFAULT 0,
    last_attempted_at TIMESTAMPTZ,
    last_error        TEXT,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE webhook_events;
DROP TYPE event_status;
