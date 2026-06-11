-- +goose Up
ALTER TABLE webhook_events
    ADD CONSTRAINT webhook_events_shopify_order_id_key UNIQUE (shopify_order_id);

-- +goose Down
ALTER TABLE webhook_events
    DROP CONSTRAINT webhook_events_shopify_order_id_key;
