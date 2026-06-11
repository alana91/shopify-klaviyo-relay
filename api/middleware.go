package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"

	"github.com/alana91/shopify-klaviyo-relay/config"
)

// Recover catches a panic from a downstream handler, logs it, and replies with
// a 500 so one bad request can't take the server down.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			slog.Error("panic recovered", "error", rec, "method", r.Method, "path", r.URL.Path)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

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
