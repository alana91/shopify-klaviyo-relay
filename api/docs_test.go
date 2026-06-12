package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleDocs(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantCT   string
		wantBody []string
	}{
		{"swagger UI page", "/swagger/index.html", "text/html", []string{"swagger-ui"}},
		{"spec document", "/swagger/doc.json", "application/json", []string{`"swagger": "2.0"`, "/webhook/shopify/orders", "/api/events"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()

			HandleDocs().ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}
			if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, tc.wantCT) {
				t.Errorf("Content-Type = %q, want prefix %q", ct, tc.wantCT)
			}
			body := rr.Body.String()
			for _, want := range tc.wantBody {
				if !strings.Contains(body, want) {
					t.Errorf("body missing %q", want)
				}
			}
		})
	}
}
