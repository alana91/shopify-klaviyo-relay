package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"time"
)

type Config struct {
	DB                   DBConfig
	ShopifyWebhookSecret string
	KlaviyoAPIKey        string
	KlaviyoBaseURL       string
	WorkerPollInterval   time.Duration
	MaxEventAge          time.Duration
	Port                 string
}

func Load() (Config, error) {
	db, err := LoadDB()
	if err != nil {
		return Config{}, err
	}

	secret, err := requireEnv("SHOPIFY_WEBHOOK_SECRET")
	if err != nil {
		return Config{}, err
	}

	klaviyoAPIKey, err := requireEnv("KLAVIYO_API_KEY")
	if err != nil {
		return Config{}, err
	}

	klaviyoBaseURL := os.Getenv("KLAVIYO_BASE_URL")
	if klaviyoBaseURL == "" {
		klaviyoBaseURL = "https://a.klaviyo.com"
	}

	pollInterval, err := durationEnv("WORKER_POLL_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	maxEventAge, err := durationEnv("MAX_EVENT_AGE", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return Config{
		DB:                   db,
		ShopifyWebhookSecret: secret,
		KlaviyoAPIKey:        klaviyoAPIKey,
		KlaviyoBaseURL:       klaviyoBaseURL,
		WorkerPollInterval:   pollInterval,
		MaxEventAge:          maxEventAge,
		Port:                 port,
	}, nil
}

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

func LoadDB() (DBConfig, error) {
	host, err := requireEnv("DB_HOST")
	if err != nil {
		return DBConfig{}, err
	}
	name, err := requireEnv("DB_NAME")
	if err != nil {
		return DBConfig{}, err
	}
	user, err := requireEnv("DB_USER")
	if err != nil {
		return DBConfig{}, err
	}
	password, err := requireEnv("DB_PASSWORD")
	if err != nil {
		return DBConfig{}, err
	}

	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}

	sslMode := os.Getenv("DB_SSLMODE")
	if sslMode == "" {
		sslMode = "prefer"
	}

	return DBConfig{Host: host, Port: port, Name: name, User: user, Password: password, SSLMode: sslMode}, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("parsing %s %q: %w", key, v, err)
	}
	return d, nil
}

func requireEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required env var %s is not set", key)
	}
	return v, nil
}

func (c DBConfig) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, c.Port),
		Path:     "/" + c.Name,
		RawQuery: "sslmode=" + c.SSLMode,
	}
	return u.String()
}
