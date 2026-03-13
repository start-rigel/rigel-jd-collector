package jdclient

import (
	"context"
	"testing"

	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
)

func TestMockClientSearchProducts(t *testing.T) {
	client := NewMockClient()
	products, err := client.SearchProducts(context.Background(), model.SearchQuery{
		Keyword:  "RTX 4060",
		Category: "GPU",
		Limit:    2,
	})
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}

	if len(products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(products))
	}

	if products[0].ExternalID == "" {
		t.Fatal("expected external id")
	}
}
