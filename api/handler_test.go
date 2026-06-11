package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/internal/testdb"
	"github.com/alana91/shopify-klaviyo-relay/store"
)

const validWebhookBody = `{"id":5678901234,"name":"#1042","email":"jane@example.com","total_price":"129.99","currency":"USD","created_at":"2026-06-11T07:00:00Z","line_items":[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]}`

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestHandleWebhookSuccess(t *testing.T) {
	ctx := context.Background()
	s, err := store.New(ctx, testdb.New(t, store.Migrate))
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook/shopify/orders", strings.NewReader(validWebhookBody))
	rr := httptest.NewRecorder()

	HandleWebhook(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID == "" {
		t.Error("response id is empty")
	}
}

func TestHandleWebhookStoreFailure(t *testing.T) {
	ctx := context.Background()
	s, err := store.New(ctx, testdb.New(t, store.Migrate))
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	_ = s.Close() // force InsertEvent to fail

	req := httptest.NewRequest(http.MethodPost, "/webhook/shopify/orders", strings.NewReader(validWebhookBody))
	rr := httptest.NewRecorder()

	HandleWebhook(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleEvents(t *testing.T) {
	ctx := context.Background()
	dsn := testdb.New(t, store.Migrate)

	s, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	defer func() { _ = s.Close() }()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open seed connection: %v", err)
	}
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	testdb.SeedEvent(t, db, testdb.Event{
		ShopifyOrderID: 1002,
		OrderName:      "#1002",
		CustomerEmail:  "bob@example.com",
		TotalPrice:     "149.00",
		Currency:       "USD",
		OrderedAt:      createdAt,
		Status:         string(store.StatusFailed),
		RetryCount:     3,
		LastError:      "klaviyo returned 400",
		CreatedAt:      createdAt,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rr := httptest.NewRecorder()

	HandleEvents(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var got struct {
		Events []struct {
			OrderID    int64     `json:"order_id"`
			Status     string    `json:"status"`
			RetryCount int       `json:"retry_count"`
			LastError  string    `json:"last_error"`
			CreatedAt  time.Time `json:"created_at"`
		} `json:"events"`
		Page  int `json:"page"`
		Limit int `json:"limit"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Page != 1 {
		t.Errorf("page = %d, want 1 (default)", got.Page)
	}
	if got.Limit != 50 {
		t.Errorf("limit = %d, want 50 (default)", got.Limit)
	}
	if got.Total != 1 {
		t.Errorf("total = %d, want 1", got.Total)
	}
	if len(got.Events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(got.Events))
	}

	e := got.Events[0]
	if e.OrderID != 1002 {
		t.Errorf("order_id = %d, want 1002", e.OrderID)
	}
	if e.Status != string(store.StatusFailed) {
		t.Errorf("status = %q, want %q", e.Status, store.StatusFailed)
	}
	if e.RetryCount != 3 {
		t.Errorf("retry_count = %d, want 3", e.RetryCount)
	}
	if e.LastError != "klaviyo returned 400" {
		t.Errorf("last_error = %q, want %q", e.LastError, "klaviyo returned 400")
	}
	if !e.CreatedAt.Equal(createdAt) {
		t.Errorf("created_at = %v, want %v", e.CreatedAt, createdAt)
	}
}

func TestHandleEventsPagination(t *testing.T) {
	ctx := context.Background()
	dsn := testdb.New(t, store.Migrate)

	s, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	defer func() { _ = s.Close() }()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open seed connection: %v", err)
	}
	defer func() { _ = db.Close() }()

	base := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	// Newest-first order is 3, 2, 1.
	for i, createdAt := range []time.Time{base.Add(-2 * time.Hour), base.Add(-1 * time.Hour), base} {
		testdb.SeedEvent(t, db, testdb.Event{
			ShopifyOrderID: int64(i + 1),
			OrderName:      "#x",
			CustomerEmail:  "jane@example.com",
			TotalPrice:     "10.00",
			Currency:       "USD",
			OrderedAt:      base,
			CreatedAt:      createdAt,
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/events?page=2&limit=1", nil)
	rr := httptest.NewRecorder()

	HandleEvents(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var got struct {
		Events []struct {
			OrderID int64 `json:"order_id"`
		} `json:"events"`
		Page  int `json:"page"`
		Limit int `json:"limit"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Page != 2 {
		t.Errorf("page = %d, want 2", got.Page)
	}
	if got.Limit != 1 {
		t.Errorf("limit = %d, want 1", got.Limit)
	}
	if got.Total != 3 {
		t.Errorf("total = %d, want 3", got.Total)
	}
	if len(got.Events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(got.Events))
	}
	if got.Events[0].OrderID != 2 {
		t.Errorf("events[0].order_id = %d, want 2", got.Events[0].OrderID)
	}
}

func TestHandleEventsClampsParams(t *testing.T) {
	ctx := context.Background()
	s, err := store.New(ctx, testdb.New(t, store.Migrate))
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	defer func() { _ = s.Close() }()

	tests := []struct {
		name      string
		query     string
		wantPage  int
		wantLimit int
	}{
		{"limit over max is capped", "?limit=10000", 1, 100},
		{"page below one is clamped", "?page=0", 1, 50},
		{"unparseable falls back to defaults", "?page=abc&limit=xyz", 1, 50},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/events"+tc.query, nil)
			rr := httptest.NewRecorder()

			HandleEvents(s).ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}

			var got struct {
				Page  int `json:"page"`
				Limit int `json:"limit"`
			}
			if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got.Page != tc.wantPage {
				t.Errorf("page = %d, want %d", got.Page, tc.wantPage)
			}
			if got.Limit != tc.wantLimit {
				t.Errorf("limit = %d, want %d", got.Limit, tc.wantLimit)
			}
		})
	}
}

func TestHandleEventsEmpty(t *testing.T) {
	ctx := context.Background()
	s, err := store.New(ctx, testdb.New(t, store.Migrate))
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	defer func() { _ = s.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rr := httptest.NewRecorder()

	HandleEvents(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if body := rr.Body.String(); !strings.Contains(body, `"events":[]`) {
		t.Errorf("body = %q, want events to be [] (not null)", body)
	}

	var got struct {
		Total int `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Total != 0 {
		t.Errorf("total = %d, want 0", got.Total)
	}
}

func TestHandleIndex(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	HandleIndex().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body := rr.Body.String()
	for _, want := range []string{"Order ID", "Status", "Retries", "Last Error", "Created At", "/api/events"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestHandleWebhookBadRequest(t *testing.T) {
	// These fail before the store is reached, so they need no DB.
	tests := []struct {
		name string
		body io.Reader
	}{
		{"unreadable body", failingReader{}},
		{"invalid JSON", strings.NewReader(`{not valid json`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/webhook/shopify/orders", tc.body)
			rr := httptest.NewRecorder()

			HandleWebhook(nil).ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
			}
		})
	}
}
