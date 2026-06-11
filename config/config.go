package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
)

type Config struct {
	DB                   DBConfig
	ShopifyWebhookSecret string
	KlaviyoAPIKey        string
	KlaviyoBaseURL       string
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return Config{
		DB:                   db,
		ShopifyWebhookSecret: secret,
		KlaviyoAPIKey:        klaviyoAPIKey,
		KlaviyoBaseURL:       klaviyoBaseURL,
		Port:                 port,
	}, nil
}

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
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

	return DBConfig{Host: host, Port: port, Name: name, User: user, Password: password}, nil
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
		RawQuery: "sslmode=disable",
	}
	return u.String()
}
