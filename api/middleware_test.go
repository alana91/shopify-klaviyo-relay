package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alana91/shopify-klaviyo-relay/config"
)

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestVerifyShopifyHMAC(t *testing.T) {
	const secret = "test-secret"
	const body = `{"id":1,"name":"#1001"}`

	cfg := &config.Config{ShopifyWebhookSecret: secret}

	tests := []struct {
		name     string
		sig      string
		wantCode int
	}{
		{
			name:     "valid signature passes",
			sig:      sign(secret, body),
			wantCode: http.StatusOK,
		},
		{
			name:     "missing header returns 401",
			sig:      "",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "invalid signature returns 401",
			sig:      sign("wrong-secret", body),
			wantCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			req.Header.Set("X-Shopify-Hmac-SHA256", tc.sig)
			rr := httptest.NewRecorder()

			VerifyShopifyHMAC(cfg, http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

			if rr.Code != tc.wantCode {
				t.Errorf("got %d, want %d", rr.Code, tc.wantCode)
			}
		})
	}

	t.Run("unreadable body returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", errReader{})
		req.Header.Set("X-Shopify-Hmac-SHA256", sign(secret, body))
		rr := httptest.NewRecorder()

		VerifyShopifyHMAC(cfg, http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("got %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}
