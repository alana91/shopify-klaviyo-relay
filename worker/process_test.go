package worker

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/alana91/shopify-klaviyo-relay/internal/testdb"
	"github.com/alana91/shopify-klaviyo-relay/store"
)

func TestProcessPendingSucceeds(t *testing.T) {
	ctx := context.Background()
	dsn := testdb.New(t, store.Migrate)

	s, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	db := openDB(t, dsn)
	id := seedReceived(t, db, store.Event{
		ShopifyOrderID: 5678901234,
		OrderName:      "#1042",
		CustomerEmail:  "jane@example.com",
		TotalPrice:     "129.99",
		Currency:       "USD",
		LineItems:      []byte(`[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]`),
		OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	wk := NewWorker(s, NewKlaviyoClient(srv.URL, "test-key"))
	if err := wk.processPending(ctx); err != nil {
		t.Fatalf("processPending() error = %v", err)
	}

	if got := readStatus(t, db, id); got != string(store.StatusSucceeded) {
		t.Errorf("status = %q, want %q", got, store.StatusSucceeded)
	}
}

func TestProcessPendingFails(t *testing.T) {
	ctx := context.Background()
	dsn := testdb.New(t, store.Migrate)

	s, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	db := openDB(t, dsn)
	id := seedReceived(t, db, store.Event{
		ShopifyOrderID: 5678901234,
		OrderName:      "#1042",
		CustomerEmail:  "jane@example.com",
		TotalPrice:     "129.99",
		Currency:       "USD",
		LineItems:      []byte(`[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]`),
		OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"errors":[{"detail":"Invalid input."}]}`)
	}))
	defer srv.Close()

	wk := NewWorker(s, NewKlaviyoClient(srv.URL, "test-key"))
	if err := wk.processPending(ctx); err != nil {
		t.Fatalf("processPending() error = %v", err)
	}

	if got := readStatus(t, db, id); got != string(store.StatusFailed) {
		t.Errorf("status = %q, want %q", got, store.StatusFailed)
	}
	if got := readLastError(t, db, id); !strings.Contains(got, "400") {
		t.Errorf("last_error = %q, want it to contain 400", got)
	}
}

func openDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// seedReceived inserts a row directly (not via store.InsertEvent) so the
// worker test stays independent of the ingestion path, and returns its id.
func seedReceived(t *testing.T, db *sql.DB, e store.Event) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO webhook_events
		    (shopify_order_id, order_name, customer_email, total_price, currency, line_items, ordered_at, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'received')
		 RETURNING id`,
		e.ShopifyOrderID, e.OrderName, e.CustomerEmail, e.TotalPrice, e.Currency, e.LineItems, e.OrderedAt).Scan(&id)
	if err != nil {
		t.Fatalf("seed received: %v", err)
	}
	return id
}

func readStatus(t *testing.T, db *sql.DB, id string) string {
	t.Helper()
	var status string
	if err := db.QueryRowContext(context.Background(),
		"SELECT status FROM webhook_events WHERE id = $1", id).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return status
}

func readLastError(t *testing.T, db *sql.DB, id string) string {
	t.Helper()
	var lastError sql.NullString
	if err := db.QueryRowContext(context.Background(),
		"SELECT last_error FROM webhook_events WHERE id = $1", id).Scan(&lastError); err != nil {
		t.Fatalf("read last_error: %v", err)
	}
	return lastError.String
}
