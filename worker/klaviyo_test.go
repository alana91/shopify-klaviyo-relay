package worker

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKlaviyoSend(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotRevision string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotRevision = r.Header.Get("revision")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	c := NewKlaviyoClient(srv.URL, "test-key")
	payload := []byte(`{"data":{"type":"event"}}`)

	if err := c.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/events/" {
		t.Errorf("path = %q, want /api/events/", gotPath)
	}
	if gotAuth != "Klaviyo-API-Key test-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Klaviyo-API-Key test-key")
	}
	if gotRevision != "2023-10-15" {
		t.Errorf("revision = %q, want %q", gotRevision, "2023-10-15")
	}
	if string(gotBody) != string(payload) {
		t.Errorf("body = %q, want %q", gotBody, payload)
	}
}

func TestKlaviyoSendFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"errors":[{"detail":"Invalid input."}]}`)
	}))
	defer srv.Close()

	c := NewKlaviyoClient(srv.URL, "test-key")

	err := c.Send(context.Background(), []byte(`{"data":{"type":"event"}}`))
	if err == nil {
		t.Fatal("Send() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want it to contain status 400", err.Error())
	}
}
