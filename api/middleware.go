package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"

	"github.com/alana91/shopify-klaviyo-relay/config"
)

func VerifyShopifyHMAC(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}

		header := r.Header.Get("X-Shopify-Hmac-SHA256")
		if header == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		sig, err := base64.StdEncoding.DecodeString(header)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		mac := hmac.New(sha256.New, []byte(cfg.ShopifyWebhookSecret))
		mac.Write(body)
		expected := mac.Sum(nil)

		if !hmac.Equal(sig, expected) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}
