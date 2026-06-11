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

func TestPendingEvents(t *testing.T) {
	t.Run("returns received events", func(t *testing.T) {
		ctx := context.Background()
		s := newTestStore(t)

		const email = "jane@example.com"
		id := testdb.SeedEvent(t, s.db, testdb.Event{
			ShopifyOrderID: 5678901234,
			OrderName:      "#1042",
			CustomerEmail:  email,
			TotalPrice:     "129.99",
			Currency:       "USD",
			LineItems:      []byte(`[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]`),
			OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
		})

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
		if pending[0].CustomerEmail != email {
			t.Errorf("CustomerEmail = %q, want %q", pending[0].CustomerEmail, email)
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
	recentID := testdb.SeedEvent(t, s.db, testdb.Event{
		ShopifyOrderID: 1,
		OrderName:      "#recent",
		CustomerEmail:  "jane@example.com",
		OrderedAt:      base,
	})
	testdb.SeedEvent(t, s.db, testdb.Event{
		ShopifyOrderID: 2,
		OrderName:      "#old",
		CustomerEmail:  "jane@example.com",
		OrderedAt:      base.Add(-48 * time.Hour),
	})

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

	for i, status := range []Status{StatusSucceeded, StatusFailed, StatusExpired} {
		testdb.SeedEvent(t, s.db, testdb.Event{
			ShopifyOrderID: int64(i + 1),
			OrderName:      "#1042",
			CustomerEmail:  "jane@example.com",
			OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
			Status:         string(status),
		})
	}

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

	id := testdb.SeedEvent(t, s.db, testdb.Event{
		ShopifyOrderID: 5678901234,
		OrderName:      "#1042",
		CustomerEmail:  "jane@example.com",
		TotalPrice:     "129.99",
		Currency:       "USD",
		OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
	})

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

	id := testdb.SeedEvent(t, s.db, testdb.Event{
		ShopifyOrderID: 5678901234,
		OrderName:      "#1042",
		CustomerEmail:  "jane@example.com",
		TotalPrice:     "129.99",
		Currency:       "USD",
		OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
	})

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

func TestListEvents(t *testing.T) {
	t.Run("returns a summary of a stored event", func(t *testing.T) {
		ctx := context.Background()
		s := newTestStore(t)

		const orderID = int64(5678901234)
		testdb.SeedEvent(t, s.db, testdb.Event{
			ShopifyOrderID: orderID,
			OrderName:      "#1042",
			CustomerEmail:  "jane@example.com",
			TotalPrice:     "129.99",
			Currency:       "USD",
			LineItems:      []byte(`[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]`),
			OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
		})

		events, err := s.ListEvents(ctx, 100, 0)
		if err != nil {
			t.Fatalf("ListEvents() error = %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("len(events) = %d, want 1", len(events))
		}

		got := events[0]
		if got.ShopifyOrderID != orderID {
			t.Errorf("ShopifyOrderID = %d, want %d", got.ShopifyOrderID, orderID)
		}
		if got.Status != StatusReceived {
			t.Errorf("Status = %q, want %q", got.Status, StatusReceived)
		}
		if got.RetryCount != 0 {
			t.Errorf("RetryCount = %d, want 0", got.RetryCount)
		}
		if got.LastError != "" {
			t.Errorf("LastError = %q, want \"\"", got.LastError)
		}
		if got.CreatedAt.IsZero() {
			t.Error("CreatedAt is zero, want non-zero")
		}
	})
}

func TestListEventsOrdersNewestFirst(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	base := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	testdb.SeedEvent(t, s.db, testdb.Event{ShopifyOrderID: 1, CreatedAt: base.Add(-2 * time.Hour)})
	testdb.SeedEvent(t, s.db, testdb.Event{ShopifyOrderID: 2, CreatedAt: base})
	testdb.SeedEvent(t, s.db, testdb.Event{ShopifyOrderID: 3, CreatedAt: base.Add(-1 * time.Hour)})

	events, err := s.ListEvents(ctx, 100, 0)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}

	want := []int64{2, 3, 1}
	for i, w := range want {
		if events[i].ShopifyOrderID != w {
			t.Errorf("events[%d].ShopifyOrderID = %d, want %d", i, events[i].ShopifyOrderID, w)
		}
	}
}

func TestListEventsPaginates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	base := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	// Newest-first order is 4, 3, 2, 1.
	testdb.SeedEvent(t, s.db, testdb.Event{ShopifyOrderID: 1, CreatedAt: base.Add(-3 * time.Hour)})
	testdb.SeedEvent(t, s.db, testdb.Event{ShopifyOrderID: 2, CreatedAt: base.Add(-2 * time.Hour)})
	testdb.SeedEvent(t, s.db, testdb.Event{ShopifyOrderID: 3, CreatedAt: base.Add(-1 * time.Hour)})
	testdb.SeedEvent(t, s.db, testdb.Event{ShopifyOrderID: 4, CreatedAt: base})

	page, err := s.ListEvents(ctx, 2, 2)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}

	want := []int64{2, 1}
	if len(page) != len(want) {
		t.Fatalf("len(page) = %d, want %d", len(page), len(want))
	}
	for i, w := range want {
		if page[i].ShopifyOrderID != w {
			t.Errorf("page[%d].ShopifyOrderID = %d, want %d", i, page[i].ShopifyOrderID, w)
		}
	}
}

func TestCountEvents(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	for i, status := range []Status{StatusReceived, StatusSucceeded, StatusFailed} {
		testdb.SeedEvent(t, s.db, testdb.Event{
			ShopifyOrderID: int64(i + 1),
			OrderName:      "#1",
			CustomerEmail:  "jane@example.com",
			OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
			Status:         string(status),
		})
	}

	total, err := s.CountEvents(ctx)
	if err != nil {
		t.Fatalf("CountEvents() error = %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
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

func TestInsertEventIsIdempotent(t *testing.T) {
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

	first, err := s.InsertEvent(ctx, event)
	if err != nil {
		t.Fatalf("first InsertEvent() error = %v", err)
	}

	second, err := s.InsertEvent(ctx, event)
	if err != nil {
		t.Fatalf("second InsertEvent() error = %v", err)
	}

	if second != first {
		t.Errorf("second id = %q, want %q (same row)", second, first)
	}

	total, err := s.CountEvents(ctx)
	if err != nil {
		t.Fatalf("CountEvents() error = %v", err)
	}
	if total != 1 {
		t.Errorf("row count = %d, want 1 (duplicate order should not create a new row)", total)
	}
}
