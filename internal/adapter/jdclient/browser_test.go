package jdclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
)

func TestBrowserClientSearchProducts(t *testing.T) {
	client := NewBrowserClient("http://browser-worker.local")
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/v1/search" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
			"mode":"public_search",
			"risk_detected":false,
			"session_required":false,
			"page_url":"https://search.jd.com/Search?keyword=RTX+4060&enc=utf-8",
			"products":[
				{
					"external_id":"1001",
					"sku_id":"1001",
					"title":"RTX 4060 官方自营 标准版",
					"subtitle":"GPU 搜索结果",
					"url":"https://item.jd.com/1001.html",
					"image_url":"https://img10.360buyimg.com/a.jpg",
					"shop_name":"京东自营",
					"shop_type":"self_operated",
					"price":1999,
					"currency":"CNY",
					"availability":"in_stock"
				}
			]
		}`)),
			}, nil
		}),
	}
	products, err := client.SearchProducts(context.Background(), model.SearchQuery{
		Keyword:  "RTX 4060",
		Category: "GPU",
		Limit:    1,
	})
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
	if products[0].SourcePlatform != model.PlatformJD {
		t.Fatalf("expected source platform jd, got %s", products[0].SourcePlatform)
	}
	if products[0].RawPayload["browser_collector_mode"] != "public_search" {
		t.Fatalf("expected browser_collector_mode public_search, got %+v", products[0].RawPayload["browser_collector_mode"])
	}
}

func TestBrowserClientSearchProductsRetriesServerError(t *testing.T) {
	attempts := 0
	client := NewBrowserClient("http://browser-worker.local")
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"error":"temporary upstream failure"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"mode":"public_search",
					"risk_detected":false,
					"session_required":false,
					"page_url":"https://search.jd.com/Search?keyword=RTX+4060&enc=utf-8",
					"products":[{"external_id":"1001","sku_id":"1001","title":"RTX 4060","url":"https://item.jd.com/1001.html","shop_name":"京东自营","shop_type":"self_operated","price":1999,"currency":"CNY","availability":"in_stock"}]
				}`)),
			}, nil
		}),
	}

	products, err := client.SearchProducts(context.Background(), model.SearchQuery{
		Keyword:  "RTX 4060",
		Category: "GPU",
		Limit:    1,
	})
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
}

func TestBrowserClientSearchProductsIncludesErrorBody(t *testing.T) {
	client := NewBrowserClient("http://browser-worker.local")
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"invalid keyword"}`)),
			}, nil
		}),
	}

	_, err := client.SearchProducts(context.Background(), model.SearchQuery{
		Keyword:  "RTX 4060",
		Category: "GPU",
		Limit:    1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid keyword") {
		t.Fatalf("expected error body in message, got %v", err)
	}
}

func TestBrowserClientSearchProductsRetriesTransportError(t *testing.T) {
	attempts := 0
	client := NewBrowserClient("http://browser-worker.local")
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return nil, fmt.Errorf("temporary network error")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"mode":"public_search",
					"risk_detected":false,
					"session_required":false,
					"page_url":"https://search.jd.com/Search?keyword=RTX+4060&enc=utf-8",
					"products":[{"external_id":"1001","sku_id":"1001","title":"RTX 4060","url":"https://item.jd.com/1001.html","shop_name":"京东自营","shop_type":"self_operated","price":1999,"currency":"CNY","availability":"in_stock"}]
				}`)),
			}, nil
		}),
	}

	products, err := client.SearchProducts(context.Background(), model.SearchQuery{
		Keyword:  "RTX 4060",
		Category: "GPU",
		Limit:    1,
	})
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
