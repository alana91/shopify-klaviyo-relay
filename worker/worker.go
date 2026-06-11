package worker

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/store"
)

const placedOrderMetric = "Placed Order"

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

	return json.Marshal(p)
}
