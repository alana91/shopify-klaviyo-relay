package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alana91/shopify-klaviyo-relay/store"
)

type Worker struct {
	store   *store.Store
	klaviyo *KlaviyoClient
}

func NewWorker(s *store.Store, k *KlaviyoClient) *Worker {
	return &Worker{store: s, klaviyo: k}
}

func (w *Worker) processPending(ctx context.Context) error {
	events, err := w.store.PendingEvents(ctx)
	if err != nil {
		return fmt.Errorf("fetching pending events: %w", err)
	}
	for _, e := range events {
		w.process(ctx, e)
	}
	return nil
}

func (w *Worker) process(ctx context.Context, e store.PendingEvent) {
	payload, err := buildKlaviyoPayload(e.Event)
	if err != nil {
		w.markFailed(ctx, e, fmt.Sprintf("building payload: %v", err))
		return
	}

	if err := w.klaviyo.Send(ctx, payload); err != nil {
		w.markFailed(ctx, e, err.Error())
		return
	}

	if err := w.store.MarkSucceeded(ctx, e.ID); err != nil {
		slog.Error("mark succeeded", "error", err, "id", e.ID, "shopify_order_id", e.ShopifyOrderID)
		return
	}
	slog.Info("event forwarded", "id", e.ID, "shopify_order_id", e.ShopifyOrderID)
}

func (w *Worker) markFailed(ctx context.Context, e store.PendingEvent, msg string) {
	slog.Error("forwarding event failed", "error", msg, "id", e.ID, "shopify_order_id", e.ShopifyOrderID)
	if err := w.store.MarkFailed(ctx, e.ID, msg); err != nil {
		slog.Error("mark failed", "error", err, "id", e.ID, "shopify_order_id", e.ShopifyOrderID)
	}
}
