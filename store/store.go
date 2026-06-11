package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db *sql.DB
}

// Status mirrors the event_status enum in the database schema.
type Status string

const (
	StatusReceived  Status = "received"
	StatusRetrying  Status = "retrying"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusExpired   Status = "expired"
)

type Event struct {
	ShopifyOrderID int64
	OrderName      string
	CustomerEmail  string
	TotalPrice     string
	Currency       string
	LineItems      []byte
	OrderedAt      time.Time
}

func New(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging db: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

type PendingEvent struct {
	ID string
	Event
}

func (s *Store) PendingEvents(ctx context.Context, cutoff time.Time) ([]PendingEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, shopify_order_id, order_name, customer_email, total_price, currency, line_items, ordered_at
		   FROM webhook_events
		  WHERE status = $1 AND ordered_at >= $2`, StatusReceived, cutoff)
	if err != nil {
		return nil, fmt.Errorf("querying pending events: %w", err)
	}
	defer rows.Close()

	var pending []PendingEvent
	for rows.Next() {
		var p PendingEvent
		if err := rows.Scan(&p.ID, &p.ShopifyOrderID, &p.OrderName, &p.CustomerEmail,
			&p.TotalPrice, &p.Currency, &p.LineItems, &p.OrderedAt); err != nil {
			return nil, fmt.Errorf("scanning pending event: %w", err)
		}
		pending = append(pending, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pending events: %w", err)
	}
	return pending, nil
}

func (s *Store) MarkSucceeded(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE webhook_events
		    SET status = $1, last_attempted_at = NOW(), updated_at = NOW()
		  WHERE id = $2`,
		StatusSucceeded, id)
	if err != nil {
		return fmt.Errorf("marking event succeeded: %w", err)
	}
	return nil
}

func (s *Store) MarkFailed(ctx context.Context, id, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE webhook_events
		    SET status = $1, last_attempted_at = NOW(), last_error = $2, updated_at = NOW()
		  WHERE id = $3`,
		StatusFailed, errMsg, id)
	if err != nil {
		return fmt.Errorf("marking event failed: %w", err)
	}
	return nil
}

type EventSummary struct {
	ShopifyOrderID int64
	Status         Status
	RetryCount     int
	LastError      string
	CreatedAt      time.Time
}

func (s *Store) ListEvents(ctx context.Context) ([]EventSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT shopify_order_id, status, retry_count, last_error, created_at
		   FROM webhook_events
		  ORDER BY created_at DESC
		  LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []EventSummary
	for rows.Next() {
		var (
			e         EventSummary
			lastError sql.NullString
		)
		if err := rows.Scan(&e.ShopifyOrderID, &e.Status, &e.RetryCount, &lastError, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		e.LastError = lastError.String
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating events: %w", err)
	}
	return events, nil
}

func (s *Store) InsertEvent(ctx context.Context, e Event) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO webhook_events
		    (shopify_order_id, order_name, customer_email, total_price, currency, line_items, ordered_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		e.ShopifyOrderID, e.OrderName, e.CustomerEmail, e.TotalPrice, e.Currency, e.LineItems, e.OrderedAt).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("inserting event: %w", err)
	}
	return id, nil
}
