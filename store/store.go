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
