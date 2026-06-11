package worker

import (
	"testing"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/store"
)

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

	want := `{"data":{"type":"event","attributes":{"metric":{"data":{"type":"metric","attributes":{"name":"Placed Order"}}},"profile":{"data":{"type":"profile","attributes":{"email":"jane@example.com"}}},"value":129.99,"properties":{"order_id":5678901234,"order_name":"#1042","line_items":[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}],"currency":"USD"},"time":"2026-06-11T07:00:00Z"}}}`

	got, err := buildKlaviyoPayload(e)
	if err != nil {
		t.Fatalf("buildKlaviyoPayload() error = %v", err)
	}
	if string(got) != want {
		t.Errorf("buildKlaviyoPayload() =\n%s\nwant\n%s", got, want)
	}
}
