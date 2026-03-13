package jdclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
)

// MockClient returns deterministic product candidates for local development.
type MockClient struct{}

func NewMockClient() *MockClient {
	return &MockClient{}
}

func (c *MockClient) SearchProducts(_ context.Context, query model.SearchQuery) ([]model.Product, error) {
	keyword := strings.TrimSpace(query.Keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword must not be empty")
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 3
	}
	if limit > 10 {
		limit = 10
	}

	base := []model.Product{
		{
			SourcePlatform: model.PlatformJD,
			ExternalID:     strings.ToLower(strings.ReplaceAll(keyword, " ", "-")) + "-jd-001",
			SKUID:          strings.ToLower(strings.ReplaceAll(keyword, " ", "")) + "001",
			Title:          fmt.Sprintf("%s 官方自营 标准版", keyword),
			Subtitle:       fmt.Sprintf("%s %s 模拟采集结果", strings.ToUpper(query.Category), keyword),
			URL:            fmt.Sprintf("https://mock.jd.local/product/%s-001", strings.ToLower(strings.ReplaceAll(keyword, " ", "-"))),
			ImageURL:       "https://img.mock.jd.local/placeholder-1.jpg",
			ShopName:       "京东自营",
			ShopType:       model.ShopTypeSelfOperated,
			SellerName:     "JD Self",
			Region:         "北京",
			Price:          1999.00,
			Currency:       "CNY",
			Availability:   "in_stock",
			Attributes: map[string]any{
				"keyword":  keyword,
				"category": query.Category,
				"brand":    query.Brand,
			},
			RawPayload: map[string]any{
				"mock": true,
				"rank": 1,
			},
		},
		{
			SourcePlatform: model.PlatformJD,
			ExternalID:     strings.ToLower(strings.ReplaceAll(keyword, " ", "-")) + "-jd-002",
			SKUID:          strings.ToLower(strings.ReplaceAll(keyword, " ", "")) + "002",
			Title:          fmt.Sprintf("%s 旗舰店 超频版", keyword),
			Subtitle:       fmt.Sprintf("%s 第二候选", keyword),
			URL:            fmt.Sprintf("https://mock.jd.local/product/%s-002", strings.ToLower(strings.ReplaceAll(keyword, " ", "-"))),
			ImageURL:       "https://img.mock.jd.local/placeholder-2.jpg",
			ShopName:       "品牌旗舰店",
			ShopType:       model.ShopTypeFlagship,
			SellerName:     "Brand Flagship",
			Region:         "上海",
			Price:          2099.00,
			Currency:       "CNY",
			Availability:   "in_stock",
			Attributes: map[string]any{
				"keyword":  keyword,
				"category": query.Category,
				"brand":    query.Brand,
			},
			RawPayload: map[string]any{
				"mock": true,
				"rank": 2,
			},
		},
		{
			SourcePlatform: model.PlatformJD,
			ExternalID:     strings.ToLower(strings.ReplaceAll(keyword, " ", "-")) + "-jd-003",
			SKUID:          strings.ToLower(strings.ReplaceAll(keyword, " ", "")) + "003",
			Title:          fmt.Sprintf("%s 第三方店铺 促销版", keyword),
			Subtitle:       fmt.Sprintf("%s 价格更低的候选", keyword),
			URL:            fmt.Sprintf("https://mock.jd.local/product/%s-003", strings.ToLower(strings.ReplaceAll(keyword, " ", "-"))),
			ImageURL:       "https://img.mock.jd.local/placeholder-3.jpg",
			ShopName:       "电脑硬件专营店",
			ShopType:       model.ShopTypeMarketplace,
			SellerName:     "Marketplace Seller",
			Region:         "深圳",
			Price:          1899.00,
			Currency:       "CNY",
			Availability:   "limited",
			Attributes: map[string]any{
				"keyword":  keyword,
				"category": query.Category,
				"brand":    query.Brand,
			},
			RawPayload: map[string]any{
				"mock": true,
				"rank": 3,
			},
		},
	}

	return base[:limit], nil
}
