package api

import (
	_ "embed"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/store"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
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

type eventsPage struct {
	Events []eventResponse `json:"events"`
	Page   int             `json:"page"`
	Limit  int             `json:"limit"`
	Total  int             `json:"total"`
}

func HandleEvents(s *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := queryInt(r, "page", 1, 1, 0)
		limit := queryInt(r, "limit", defaultPageLimit, 1, maxPageLimit)
		offset := (page - 1) * limit

		events, err := s.ListEvents(r.Context(), limit, offset)
		if err != nil {
			slog.Error("list events", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		total, err := s.CountEvents(r.Context())
		if err != nil {
			slog.Error("count events", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := eventsPage{
			Events: make([]eventResponse, 0, len(events)),
			Page:   page,
			Limit:  limit,
			Total:  total,
		}
		for _, e := range events {
			resp.Events = append(resp.Events, eventResponse{
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

// queryInt reads an int query param, clamping to [min, max]; max <= 0 means no
// upper bound. Missing or unparseable values fall back to def.
func queryInt(r *http.Request, key string, def, min, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if max > 0 && v > max {
		return max
	}
	return v
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
