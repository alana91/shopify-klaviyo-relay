package worker

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/store"
)

func TestKlaviyoSend(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotRevision string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotRevision = r.Header.Get("revision")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	c := NewKlaviyoClient(srv.URL, "test-key")
	payload := []byte(`{"data":{"type":"event"}}`)

	if err := c.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/events/" {
		t.Errorf("path = %q, want /api/events/", gotPath)
	}
	if gotAuth != "Klaviyo-API-Key test-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Klaviyo-API-Key test-key")
	}
	if gotRevision != "2023-10-15" {
		t.Errorf("revision = %q, want %q", gotRevision, "2023-10-15")
	}
	if string(gotBody) != string(payload) {
		t.Errorf("body = %q, want %q", gotBody, payload)
	}
}

func TestKlaviyoSendFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"errors":[{"detail":"Invalid input."}]}`)
	}))
	defer srv.Close()

	c := NewKlaviyoClient(srv.URL, "test-key")

	err := c.Send(context.Background(), []byte(`{"data":{"type":"event"}}`))
	if err == nil {
		t.Fatal("Send() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want it to contain status 400", err.Error())
	}
}

func TestBuildKlaviyoPayload(t *testing.T) {
	e := store.Event{
		ShopifyOrderID: 5678901234,
		OrderName:      "#1042",
		CustomerEmail:  "jane@example.com",
		TotalPrice:     "129.99",
		Currency:       "USD",
		LineItems:      []byte(`[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]`),
		OrderedAt:      time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
	}

	want := `{"data":{"type":"event","attributes":{"metric":{"data":{"type":"metric","attributes":{"name":"Placed Order"}}},"profile":{"data":{"type":"profile","attributes":{"email":"jane@example.com"}}},"value":129.99,"properties":{"order_id":5678901234,"order_name":"#1042","line_items":[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}],"currency":"USD"},"time":"2026-06-11T07:00:00Z","unique_id":"placed-order-5678901234"}}}`

	got, err := buildKlaviyoPayload(e)
	if err != nil {
		t.Fatalf("buildKlaviyoPayload() error = %v", err)
	}
	if string(got) != want {
		t.Errorf("buildKlaviyoPayload() =\n%s\nwant\n%s", got, want)
	}
}
