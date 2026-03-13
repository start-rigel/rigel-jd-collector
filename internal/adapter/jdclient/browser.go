package jdclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
)

type BrowserClient struct {
	baseURL    string
	httpClient *http.Client
}

const (
	browserClientMaxAttempts = 3
	browserClientRetryDelay  = 500 * time.Millisecond
)

type browserSearchResponse struct {
	Mode            string                 `json:"mode"`
	RiskDetected    bool                   `json:"risk_detected"`
	SessionRequired bool                   `json:"session_required"`
	PageURL         string                 `json:"page_url"`
	FallbackReason  string                 `json:"fallback_reason,omitempty"`
	FallbackDetails map[string]any         `json:"fallback_details,omitempty"`
	Products        []browserSearchProduct `json:"products"`
}

type browserSearchProduct struct {
	ExternalID   string  `json:"external_id"`
	SKUID        string  `json:"sku_id"`
	Title        string  `json:"title"`
	Subtitle     string  `json:"subtitle"`
	URL          string  `json:"url"`
	ImageURL     string  `json:"image_url"`
	ShopName     string  `json:"shop_name"`
	ShopType     string  `json:"shop_type"`
	Price        float64 `json:"price"`
	Currency     string  `json:"currency"`
	Availability string  `json:"availability"`
}

func NewBrowserClient(baseURL string) *BrowserClient {
	return &BrowserClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *BrowserClient) SearchProducts(ctx context.Context, query model.SearchQuery) ([]model.Product, error) {
	if strings.TrimSpace(query.Keyword) == "" {
		return nil, fmt.Errorf("keyword must not be empty")
	}

	requestPayload := map[string]any{
		"keyword":  query.Keyword,
		"category": query.Category,
		"brand":    query.Brand,
		"limit":    query.Limit,
	}
	encoded, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal browser collector request: %w", err)
	}

	body, err := c.searchWithRetry(ctx, encoded)
	if err != nil {
		return nil, err
	}

	var payload browserSearchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode browser collector response: %w", err)
	}

	products := make([]model.Product, 0, len(payload.Products))
	for _, item := range payload.Products {
		products = append(products, model.Product{
			SourcePlatform: model.PlatformJD,
			ExternalID:     item.ExternalID,
			SKUID:          item.SKUID,
			Title:          item.Title,
			Subtitle:       item.Subtitle,
			URL:            item.URL,
			ImageURL:       item.ImageURL,
			ShopName:       item.ShopName,
			ShopType:       model.ShopType(item.ShopType),
			Price:          item.Price,
			Currency:       blankFallback(item.Currency, "CNY"),
			Availability:   blankFallback(item.Availability, "unknown"),
			Attributes: map[string]any{
				"category": query.Category,
				"brand":    query.Brand,
				"keyword":  query.Keyword,
			},
			RawPayload: map[string]any{
				"browser_collector_mode": payload.Mode,
				"risk_detected":          payload.RiskDetected,
				"session_required":       payload.SessionRequired,
				"page_url":               payload.PageURL,
				"fallback_reason":        payload.FallbackReason,
				"fallback_details":       payload.FallbackDetails,
			},
		})
	}

	return products, nil
}

func (c *BrowserClient) searchWithRetry(ctx context.Context, encoded []byte) ([]byte, error) {
	var lastErr error

	for attempt := 1; attempt <= browserClientMaxAttempts; attempt++ {
		body, shouldRetry, err := c.searchOnce(ctx, encoded)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !shouldRetry || attempt == browserClientMaxAttempts {
			break
		}
		if retryErr := sleepWithContext(ctx, browserClientRetryDelay); retryErr != nil {
			return nil, lastErr
		}
	}

	return nil, lastErr
}

func (c *BrowserClient) searchOnce(ctx context.Context, encoded []byte) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/search", bytes.NewReader(encoded))
	if err != nil {
		return nil, false, fmt.Errorf("create browser collector request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("request browser collector: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if readErr != nil {
		return nil, true, fmt.Errorf("read browser collector response: %w", readErr)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		message := strings.TrimSpace(string(body))
		if message != "" {
			return nil, resp.StatusCode >= http.StatusInternalServerError, fmt.Errorf("browser collector returned status %d: %s", resp.StatusCode, message)
		}
		return nil, resp.StatusCode >= http.StatusInternalServerError, fmt.Errorf("browser collector returned status %d", resp.StatusCode)
	}

	return body, false, nil
}

func blankFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
