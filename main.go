package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/alana91/shopify-klaviyo-relay/api"
	"github.com/alana91/shopify-klaviyo-relay/config"
	"github.com/alana91/shopify-klaviyo-relay/store"
	"github.com/alana91/shopify-klaviyo-relay/worker"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	s, err := store.New(ctx, cfg.DB.DSN())
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	if err := s.Migrate(ctx); err != nil {
		return err
	}
	slog.Info("migrations applied")

	klaviyo := worker.NewKlaviyoClient(cfg.KlaviyoBaseURL, cfg.KlaviyoAPIKey)
	wk := worker.NewWorker(s, klaviyo, cfg.MaxEventAge)
	go func() {
		if err := wk.Run(ctx, cfg.WorkerPollInterval); err != nil {
			slog.Info("worker exited", "reason", err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("POST /webhook/shopify/orders", api.VerifyShopifyHMAC(&cfg, api.HandleWebhook(s)))
	mux.Handle("GET /api/events", api.HandleEvents(s))
	mux.Handle("GET /{$}", api.HandleIndex())

	addr := ":" + cfg.Port
	slog.Info("listening", "addr", addr)
	return http.ListenAndServe(addr, mux)
}
