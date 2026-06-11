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
	_, err = db.ExecContext(ctx,
		`INSERT INTO webhook_events
		    (shopify_order_id, order_name, customer_email, total_price, currency, line_items, ordered_at, status, retry_count, last_error, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		int64(1002), "#1002", "bob@example.com", "149.00", "USD", []byte(`[]`),
		createdAt, store.StatusFailed, 3, "klaviyo returned 400", createdAt)
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rr := httptest.NewRecorder()

	HandleEvents(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var got []struct {
		OrderID    int64     `json:"order_id"`
		Status     string    `json:"status"`
		RetryCount int       `json:"retry_count"`
		LastError  string    `json:"last_error"`
		CreatedAt  time.Time `json:"created_at"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(got))
	}

	e := got[0]
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
	if body := strings.TrimSpace(rr.Body.String()); body != "[]" {
		t.Errorf("body = %q, want %q", body, "[]")
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
