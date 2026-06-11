package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/store"
)

const (
	klaviyoRevision   = "2023-10-15"
	placedOrderMetric = "Placed Order"
)

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

type klaviyoPayload struct {
	Data klaviyoData `json:"data"`
}

type klaviyoData struct {
	Type       string            `json:"type"`
	Attributes klaviyoAttributes `json:"attributes"`
}

type klaviyoAttributes struct {
	Metric     klaviyoMetric     `json:"metric"`
	Profile    klaviyoProfile    `json:"profile"`
	Value      float64           `json:"value"`
	Properties klaviyoProperties `json:"properties"`
	Time       time.Time         `json:"time"`
	UniqueID   string            `json:"unique_id"`
}

type klaviyoMetric struct {
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			Name string `json:"name"`
		} `json:"attributes"`
	} `json:"data"`
}

type klaviyoProfile struct {
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			Email string `json:"email"`
		} `json:"attributes"`
	} `json:"data"`
}

type klaviyoProperties struct {
	OrderID   int64           `json:"order_id"`
	OrderName string          `json:"order_name"`
	LineItems json.RawMessage `json:"line_items"`
	Currency  string          `json:"currency"`
}

func buildKlaviyoPayload(e store.Event) ([]byte, error) {
	value, err := strconv.ParseFloat(e.TotalPrice, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing total price %q: %w", e.TotalPrice, err)
	}

	var p klaviyoPayload
	p.Data.Type = "event"
	a := &p.Data.Attributes
	a.Metric.Data.Type = "metric"
	a.Metric.Data.Attributes.Name = placedOrderMetric
	a.Profile.Data.Type = "profile"
	a.Profile.Data.Attributes.Email = e.CustomerEmail
	a.Value = value
	a.Properties = klaviyoProperties{
		OrderID:   e.ShopifyOrderID,
		OrderName: e.OrderName,
		LineItems: json.RawMessage(e.LineItems),
		Currency:  e.Currency,
	}
	a.Time = e.OrderedAt
	a.UniqueID = fmt.Sprintf("placed-order-%d", e.ShopifyOrderID)

	return json.Marshal(p)
}
