package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/config"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func New(t *testing.T, migrate func(context.Context, *sql.DB) error) string {
	t.Helper()
	ctx := context.Background()

	dbCfg, err := config.LoadDB()
	if err != nil {
		t.Fatalf("load db config: %v", err)
	}

	adminCfg := dbCfg
	adminCfg.Name = "postgres"
	admin, err := sql.Open("pgx", adminCfg.DSN())
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	if err := admin.PingContext(ctx); err != nil {
		t.Fatalf("ping admin connection: %v", err)
	}

	name := fmt.Sprintf("relay_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}

	t.Cleanup(func() {
		if _, err := admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
		_ = admin.Close()
	})

	testCfg := dbCfg
	testCfg.Name = name
	dsn := testCfg.DSN()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Close()

	return dsn
}

// Event describes a webhook_events row to seed. Fields use primitive types so
// this package stays free of a store import (store's own tests import testdb).
// Zero values fall back to sensible defaults.
type Event struct {
	ShopifyOrderID int64
	OrderName      string
	CustomerEmail  string
	TotalPrice     string // "" => "0"
	Currency       string
	LineItems      []byte // nil => []
	OrderedAt      time.Time
	Status         string // "" => received
	RetryCount     int
	LastError      string    // "" => NULL
	CreatedAt      time.Time // zero => now
}

// SeedEvent inserts a row directly (not via store.InsertEvent) so tests stay
// isolated from the ingestion code, and returns the new row's id.
func SeedEvent(t *testing.T, db *sql.DB, e Event) string {
	t.Helper()

	if e.LineItems == nil {
		e.LineItems = []byte("[]")
	}
	if e.TotalPrice == "" {
		e.TotalPrice = "0"
	}
	if e.Status == "" {
		e.Status = "received"
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	var lastError sql.NullString
	if e.LastError != "" {
		lastError = sql.NullString{String: e.LastError, Valid: true}
	}

	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO webhook_events
		    (shopify_order_id, order_name, customer_email, total_price, currency,
		     line_items, ordered_at, status, retry_count, last_error, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		e.ShopifyOrderID, e.OrderName, e.CustomerEmail, e.TotalPrice, e.Currency,
		e.LineItems, e.OrderedAt, e.Status, e.RetryCount, lastError, e.CreatedAt).Scan(&id)
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	return id
}
