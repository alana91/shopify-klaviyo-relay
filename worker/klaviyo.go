package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

const klaviyoRevision = "2023-10-15"

type KlaviyoClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewKlaviyoClient(baseURL, apiKey string) *KlaviyoClient {
	return &KlaviyoClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{},
	}
}

func (c *KlaviyoClient) Send(ctx context.Context, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/events/", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("building klaviyo request: %w", err)
	}
	req.Header.Set("Authorization", "Klaviyo-API-Key "+c.apiKey)
	req.Header.Set("revision", klaviyoRevision)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sending klaviyo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("klaviyo returned %d: %s", resp.StatusCode, body)
	}
	return nil
}
