-- Demo data for the dashboard. 60 rows so the default page size (50) overflows
-- onto a second page, cycling through every status. Truncated first so repeated
-- `make seed-db` runs stay deterministic.
TRUNCATE webhook_events;

INSERT INTO webhook_events
    (shopify_order_id, order_name, customer_email, total_price, currency,
     line_items, ordered_at, status, retry_count, last_error, created_at)
SELECT
    2000 + g,
    '#' || (2000 + g),
    'customer' || g || '@example.com',
    (10 + g)::numeric,
    'USD',
    '[{"title":"Demo Item","quantity":1,"price":"10.00"}]'::jsonb,
    NOW() - (g || ' hours')::interval,
    (ARRAY['received','retrying','succeeded','failed','expired']::event_status[])[1 + (g % 5)],
    CASE WHEN g % 5 IN (1, 3) THEN 1 + (g % 4) ELSE 0 END,
    CASE WHEN g % 5 = 3 THEN 'klaviyo returned 400: invalid email' ELSE NULL END,
    NOW() - (g || ' minutes')::interval
FROM generate_series(1, 60) AS g;
