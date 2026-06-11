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

// maxWebhookBody caps the request body the HMAC middleware will buffer. The
// whole body must be read to compute the signature, so an uncapped read lets an
// unauthenticated client exhaust memory. 1 MiB is ample for an order webhook.
const maxWebhookBody = 1 << 20

func VerifyShopifyHMAC(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBody)
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
