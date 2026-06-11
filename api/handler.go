package api

import (
	_ "embed"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/store"
)

//go:embed index.html
var indexHTML []byte

func HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write(indexHTML); err != nil {
			slog.Error("write index", "error", err)
		}
	})
}

type eventResponse struct {
	OrderID    int64     `json:"order_id"`
	Status     string    `json:"status"`
	RetryCount int       `json:"retry_count"`
	LastError  string    `json:"last_error"`
	CreatedAt  time.Time `json:"created_at"`
}

func HandleEvents(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		events, err := s.ListEvents(r.Context())
		if err != nil {
			slog.Error("list events", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := make([]eventResponse, 0, len(events))
		for _, e := range events {
			resp = append(resp, eventResponse{
				OrderID:    e.ShopifyOrderID,
				Status:     string(e.Status),
				RetryCount: e.RetryCount,
				LastError:  e.LastError,
				CreatedAt:  e.CreatedAt,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("encode events", "error", err)
		}
	})
}

func HandleWebhook(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("read webhook body", "error", err)
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}

		order, err := parseShopifyOrder(body)
		if err != nil {
			slog.Error("parse webhook body", "error", err)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		id, err := StoreOrder(r.Context(), s, order)
		if err != nil {
			slog.Error("store order", "error", err, "shopify_order_id", order.ID)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		slog.Info("webhook stored", "id", id, "shopify_order_id", order.ID)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"id": id}); err != nil {
			slog.Error("encode response", "error", err)
		}
	})
}
