package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/internal/testdb"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	s, err := New(ctx, testdb.New(t, Migrate))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = s.db.Close() })
	return s
}

// seedEvent inserts a row directly (not via InsertEvent) so tests stay
// isolated from the code under test, and returns the new row's id.
func seedEvent(t *testing.T, s *Store, e Event, status Status) string {
	t.Helper()
	var id string
	err := s.db.QueryRowContext(context.Background(),
		`INSERT INTO webhook_events
		    (shopify_order_id, order_name, customer_email, total_price, currency, line_items, ordered_at, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		e.ShopifyOrderID, e.OrderName, e.CustomerEmail, e.TotalPrice, e.Currency, e.LineItems, e.OrderedAt, status).Scan(&id)
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	return id
}

func TestPendingEvents(t *testing.T) {
	t.Run("returns received events", func(t *testing.T) {
		ctx := context.Background()
		s := newTestStore(t)

		event := Event{
			ShopifyOrderID: 5678901234,
			OrderName:      "#1042",
			CustomerEmail:  "jane@example.com",
			TotalPrice:     "129.99",
			Currency:       "USD",
			LineItems:      []byte(`[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]`),
			OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
		}
		id := seedEvent(t, s, event, StatusReceived)

		pending, err := s.PendingEvents(ctx, time.Time{})
		if err != nil {
			t.Fatalf("PendingEvents() error = %v", err)
		}
		if len(pending) != 1 {
			t.Fatalf("len(pending) = %d, want 1", len(pending))
		}
		if pending[0].ID != id {
			t.Errorf("ID = %q, want %q", pending[0].ID, id)
		}
		if pending[0].CustomerEmail != event.CustomerEmail {
			t.Errorf("CustomerEmail = %q, want %q", pending[0].CustomerEmail, event.CustomerEmail)
		}
	})
}

func TestPendingEventsEmpty(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	pending, err := s.PendingEvents(ctx, time.Time{})
	if err != nil {
		t.Fatalf("PendingEvents() error = %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("len(pending) = %d, want 0", len(pending))
	}
}

func TestPendingEventsExcludesOld(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	base := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	recent := Event{
		ShopifyOrderID: 1,
		OrderName:      "#recent",
		CustomerEmail:  "jane@example.com",
		TotalPrice:     "10.00",
		Currency:       "USD",
		LineItems:      []byte(`[]`),
		OrderedAt:      base,
	}
	old := Event{
		ShopifyOrderID: 2,
		OrderName:      "#old",
		CustomerEmail:  "jane@example.com",
		TotalPrice:     "10.00",
		Currency:       "USD",
		LineItems:      []byte(`[]`),
		OrderedAt:      base.Add(-48 * time.Hour),
	}
	recentID := seedEvent(t, s, recent, StatusReceived)
	seedEvent(t, s, old, StatusReceived)

	cutoff := base.Add(-24 * time.Hour)
	pending, err := s.PendingEvents(ctx, cutoff)
	if err != nil {
		t.Fatalf("PendingEvents() error = %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("len(pending) = %d, want 1", len(pending))
	}
	if pending[0].ID != recentID {
		t.Errorf("ID = %q, want %q", pending[0].ID, recentID)
	}
}

func TestPendingEventsExcludesTerminalStatuses(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	event := Event{
		ShopifyOrderID: 5678901234,
		OrderName:      "#1042",
		CustomerEmail:  "jane@example.com",
		TotalPrice:     "129.99",
		Currency:       "USD",
		LineItems:      []byte(`[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]`),
		OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
	}
	seedEvent(t, s, event, StatusSucceeded)
	seedEvent(t, s, event, StatusFailed)
	seedEvent(t, s, event, StatusExpired)

	pending, err := s.PendingEvents(ctx, time.Time{})
	if err != nil {
		t.Fatalf("PendingEvents() error = %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("len(pending) = %d, want 0", len(pending))
	}
}

func TestMarkSucceeded(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	event := Event{
		ShopifyOrderID: 5678901234,
		OrderName:      "#1042",
		CustomerEmail:  "jane@example.com",
		TotalPrice:     "129.99",
		Currency:       "USD",
		LineItems:      []byte(`[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]`),
		OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
	}
	id := seedEvent(t, s, event, StatusReceived)

	if err := s.MarkSucceeded(ctx, id); err != nil {
		t.Fatalf("MarkSucceeded() error = %v", err)
	}

	var status Status
	var lastAttemptedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx,
		"SELECT status, last_attempted_at FROM webhook_events WHERE id = $1", id).
		Scan(&status, &lastAttemptedAt); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != StatusSucceeded {
		t.Errorf("status = %q, want %q", status, StatusSucceeded)
	}
	if !lastAttemptedAt.Valid {
		t.Error("last_attempted_at is null, want non-null")
	}
}

func TestMarkFailed(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	event := Event{
		ShopifyOrderID: 5678901234,
		OrderName:      "#1042",
		CustomerEmail:  "jane@example.com",
		TotalPrice:     "129.99",
		Currency:       "USD",
		LineItems:      []byte(`[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]`),
		OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
	}
	id := seedEvent(t, s, event, StatusReceived)

	if err := s.MarkFailed(ctx, id, "klaviyo returned 400"); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	var status Status
	var lastAttemptedAt sql.NullTime
	var lastError sql.NullString
	if err := s.db.QueryRowContext(ctx,
		"SELECT status, last_attempted_at, last_error FROM webhook_events WHERE id = $1", id).
		Scan(&status, &lastAttemptedAt, &lastError); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != StatusFailed {
		t.Errorf("status = %q, want %q", status, StatusFailed)
	}
	if !lastAttemptedAt.Valid {
		t.Error("last_attempted_at is null, want non-null")
	}
	if lastError.String != "klaviyo returned 400" {
		t.Errorf("last_error = %q, want %q", lastError.String, "klaviyo returned 400")
	}
}

func TestInsertEvent(t *testing.T) {
	t.Run("stores a row with status received", func(t *testing.T) {
		ctx := context.Background()
		s := newTestStore(t)

		event := Event{
			ShopifyOrderID: 5678901234,
			OrderName:      "#1042",
			CustomerEmail:  "jane@example.com",
			TotalPrice:     "129.99",
			Currency:       "USD",
			LineItems:      []byte(`[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]`),
			OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
		}

		id, err := s.InsertEvent(ctx, event)
		if err != nil {
			t.Fatalf("InsertEvent() error = %v", err)
		}
		if id == "" {
			t.Fatal("InsertEvent() returned empty id")
		}

		var status string
		if err := s.db.QueryRowContext(ctx,
			"SELECT status FROM webhook_events WHERE id = $1", id).Scan(&status); err != nil {
			t.Fatalf("read back: %v", err)
		}
		if status != "received" {
			t.Errorf("status = %q, want %q", status, "received")
		}
	})
}
