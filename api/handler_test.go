package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
