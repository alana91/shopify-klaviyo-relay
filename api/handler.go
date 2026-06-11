package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/alana91/shopify-klaviyo-relay/store"
)

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
