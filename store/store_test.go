package store

import (
	"context"
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
