package config

import (
	"testing"
	"time"
)

func TestDBConfigDSN(t *testing.T) {
	t.Run("composes a postgres url", func(t *testing.T) {
		cfg := DBConfig{
			Host:     "localhost",
			Port:     "5432",
			Name:     "relay",
			User:     "relay",
			Password: "secret",
			SSLMode:  "disable",
		}
		want := "postgres://relay:secret@localhost:5432/relay?sslmode=disable"
		if got := cfg.DSN(); got != want {
			t.Errorf("DSN() = %q, want %q", got, want)
		}
	})
}

func TestLoad(t *testing.T) {
	t.Run("loads db, secret, and port", func(t *testing.T) {
		t.Setenv("DB_HOST", "localhost")
		t.Setenv("DB_PORT", "5432")
		t.Setenv("DB_NAME", "relay")
		t.Setenv("DB_USER", "relay")
		t.Setenv("DB_PASSWORD", "secret")
		t.Setenv("SHOPIFY_WEBHOOK_SECRET", "whsec_123")
		t.Setenv("KLAVIYO_API_KEY", "pk_test_456")
		t.Setenv("KLAVIYO_BASE_URL", "https://custom.example")
		t.Setenv("WORKER_POLL_INTERVAL", "10s")
		t.Setenv("MAX_EVENT_AGE", "48h")
		t.Setenv("PORT", "9090")

		want := Config{
			DB:                   DBConfig{Host: "localhost", Port: "5432", Name: "relay", User: "relay", Password: "secret", SSLMode: "prefer"},
			ShopifyWebhookSecret: "whsec_123",
			KlaviyoAPIKey:        "pk_test_456",
			KlaviyoBaseURL:       "https://custom.example",
			WorkerPollInterval:   10 * time.Second,
			MaxEventAge:          48 * time.Hour,
			Port:                 "9090",
		}
		got, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got != want {
			t.Errorf("Load() = %+v, want %+v", got, want)
		}
	})
}

func TestLoadDefaults(t *testing.T) {
	t.Run("PORT defaults to 8080", func(t *testing.T) {
		t.Setenv("DB_HOST", "localhost")
		t.Setenv("DB_NAME", "relay")
		t.Setenv("DB_USER", "relay")
		t.Setenv("DB_PASSWORD", "secret")
		t.Setenv("SHOPIFY_WEBHOOK_SECRET", "whsec_123")
		t.Setenv("KLAVIYO_API_KEY", "pk_test_456")
		t.Setenv("PORT", "")

		got, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.Port != "8080" {
			t.Errorf("Port = %q, want %q", got.Port, "8080")
		}
		if got.KlaviyoBaseURL != "https://a.klaviyo.com" {
			t.Errorf("KlaviyoBaseURL = %q, want %q", got.KlaviyoBaseURL, "https://a.klaviyo.com")
		}
		if got.WorkerPollInterval != 5*time.Second {
			t.Errorf("WorkerPollInterval = %v, want %v", got.WorkerPollInterval, 5*time.Second)
		}
		if got.MaxEventAge != 24*time.Hour {
			t.Errorf("MaxEventAge = %v, want %v", got.MaxEventAge, 24*time.Hour)
		}
	})
}

func TestLoadDB(t *testing.T) {
	t.Run("reads DB_* env vars", func(t *testing.T) {
		t.Setenv("DB_HOST", "localhost")
		t.Setenv("DB_PORT", "5433")
		t.Setenv("DB_NAME", "relay")
		t.Setenv("DB_USER", "relay")
		t.Setenv("DB_PASSWORD", "secret")

		want := DBConfig{Host: "localhost", Port: "5433", Name: "relay", User: "relay", Password: "secret", SSLMode: "prefer"}
		got, err := LoadDB()
		if err != nil {
			t.Fatalf("LoadDB() error = %v", err)
		}
		if got != want {
			t.Errorf("LoadDB() = %+v, want %+v", got, want)
		}
	})
}
