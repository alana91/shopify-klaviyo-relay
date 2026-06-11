package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/internal/testdb"
	"github.com/alana91/shopify-klaviyo-relay/store"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestParseShopifyOrder(t *testing.T) {
	payload := []byte(`{"id":5678901234,"name":"#1042","email":"jane@example.com","total_price":"129.99","currency":"USD","created_at":"2026-06-11T07:00:00Z","line_items":[{"title":"Wireless Headphones","quantity":1,"price":"129.99"}]}`)

	want := ShopifyOrder{
		ID:         5678901234,
		Name:       "#1042",
		Email:      "jane@example.com",
		TotalPrice: "129.99",
		Currency:   "USD",
		CreatedAt:  time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
		LineItems:  []LineItem{{Title: "Wireless Headphones", Quantity: 1, Price: "129.99"}},
	}

	got, err := parseShopifyOrder(payload)
	if err != nil {
		t.Fatalf("parseShopifyOrder() error = %v", err)
	}

	checks := []struct {
		field     string
		equal     bool
		got, want any
	}{
		{"ID", got.ID == want.ID, got.ID, want.ID},
		{"Name", got.Name == want.Name, got.Name, want.Name},
		{"Email", got.Email == want.Email, got.Email, want.Email},
		{"TotalPrice", got.TotalPrice == want.TotalPrice, got.TotalPrice, want.TotalPrice},
		{"Currency", got.Currency == want.Currency, got.Currency, want.Currency},
		{"CreatedAt", got.CreatedAt.Equal(want.CreatedAt), got.CreatedAt, want.CreatedAt},
		{"LineItems", slices.Equal(got.LineItems, want.LineItems), got.LineItems, want.LineItems},
	}
	for _, c := range checks {
		if !c.equal {
			t.Errorf("%s = %v, want %v", c.field, c.got, c.want)
		}
	}
}

func TestParseShopifyOrderInvalidJSON(t *testing.T) {
	if _, err := parseShopifyOrder([]byte(`{not valid json`)); err == nil {
		t.Error("parseShopifyOrder() error = nil, want error")
	}
}

func TestStoreOrder(t *testing.T) {
	t.Run("maps order and inserts a row", func(t *testing.T) {
		ctx := context.Background()
		dsn := testdb.New(t, store.Migrate)

		s, err := store.New(ctx, dsn)
		if err != nil {
			t.Fatalf("store.New() error = %v", err)
		}
		defer func() { _ = s.Close() }()

		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatalf("open read-back connection: %v", err)
		}
		defer func() { _ = db.Close() }()

		order := ShopifyOrder{
			ID:         5678901234,
			Name:       "#1042",
			Email:      "jane@example.com",
			TotalPrice: "129.99",
			Currency:   "USD",
			CreatedAt:  time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC),
			LineItems:  []LineItem{{Title: "Wireless Headphones", Quantity: 1, Price: "129.99"}},
		}

		id, err := StoreOrder(ctx, s, order)
		if err != nil {
			t.Fatalf("StoreOrder() error = %v", err)
		}

		var (
			gotOrderID  int64
			gotName     string
			gotEmail    string
			gotPrice    string
			gotCurrency string
			gotItems    []byte
			gotOrdered  time.Time
		)
		err = db.QueryRowContext(ctx,
			`SELECT shopify_order_id, order_name, customer_email, total_price, currency, line_items, ordered_at
			 FROM webhook_events WHERE id = $1`, id).
			Scan(&gotOrderID, &gotName, &gotEmail, &gotPrice, &gotCurrency, &gotItems, &gotOrdered)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}

		var gotLineItems []LineItem
		if err := json.Unmarshal(gotItems, &gotLineItems); err != nil {
			t.Fatalf("unmarshal line_items: %v", err)
		}

		checks := []struct {
			field     string
			equal     bool
			got, want any
		}{
			{"shopify_order_id", gotOrderID == order.ID, gotOrderID, order.ID},
			{"order_name", gotName == order.Name, gotName, order.Name},
			{"customer_email", gotEmail == order.Email, gotEmail, order.Email},
			{"total_price", gotPrice == order.TotalPrice, gotPrice, order.TotalPrice},
			{"currency", gotCurrency == order.Currency, gotCurrency, order.Currency},
			{"line_items", slices.Equal(gotLineItems, order.LineItems), gotLineItems, order.LineItems},
			{"ordered_at", gotOrdered.Equal(order.CreatedAt), gotOrdered, order.CreatedAt},
		}
		for _, c := range checks {
			if !c.equal {
				t.Errorf("%s = %v, want %v", c.field, c.got, c.want)
			}
		}
	})
}
