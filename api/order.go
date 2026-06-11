package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/store"
)

type LineItem struct {
	Title    string `json:"title"`
	Quantity int    `json:"quantity"`
	Price    string `json:"price"`
}

type ShopifyOrder struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	TotalPrice string     `json:"total_price"`
	Currency   string     `json:"currency"`
	CreatedAt  time.Time  `json:"created_at"`
	LineItems  []LineItem `json:"line_items"`
}

func parseShopifyOrder(data []byte) (ShopifyOrder, error) {
	var order ShopifyOrder
	if err := json.Unmarshal(data, &order); err != nil {
		return ShopifyOrder{}, fmt.Errorf("parsing shopify order: %w", err)
	}
	return order, nil
}

func StoreOrder(ctx context.Context, s *store.Store, order ShopifyOrder) (string, error) {
	lineItems, err := json.Marshal(order.LineItems)
	if err != nil {
		return "", fmt.Errorf("marshaling line items: %w", err)
	}

	return s.InsertEvent(ctx, store.Event{
		ShopifyOrderID: order.ID,
		OrderName:      order.Name,
		CustomerEmail:  order.Email,
		TotalPrice:     order.TotalPrice,
		Currency:       order.Currency,
		LineItems:      lineItems,
		OrderedAt:      order.CreatedAt,
	})
}
