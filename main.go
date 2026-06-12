package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/api"
	"github.com/alana91/shopify-klaviyo-relay/config"
	"github.com/alana91/shopify-klaviyo-relay/store"
	"github.com/alana91/shopify-klaviyo-relay/worker"
)

const shutdownTimeout = 10 * time.Second

// @title			Shopify Klaviyo Relay
// @version		1.0
// @description	Receives Shopify order webhooks and relays them to Klaviyo as Placed Order events, with retries.
// @BasePath		/
//
// @securityDefinitions.apikey	ShopifyHmac
// @in							header
// @name						X-Shopify-Hmac-SHA256
// @description				Base64 HMAC-SHA256 signature of the raw request body, computed with the shared webhook secret. Not a static key — Swagger UI cannot generate it; use scripts/send_webhook.sh to send a signed request.
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := wk.Run(ctx, cfg.WorkerPollInterval); err != nil {
			slog.Info("worker exited", "reason", err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("POST /webhook/shopify/orders", api.VerifyShopifyHMAC(&cfg, api.HandleWebhook(s)))
	mux.Handle("GET /api/events", api.HandleEvents(s))
	mux.Handle("GET /{$}", api.HandleIndex())
	mux.Handle("GET /swagger/", api.HandleDocs())

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           api.Recover(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown", "error", err)
	}
	wg.Wait()
	slog.Info("shutdown complete")
	return nil
}
