package collector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rigel-labs/rigel-jd-collector/internal/adapter/jdclient"
	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
)

type memoryRepo struct {
	job      model.Job
	jobCount int
	products []model.Product
}

func (r *memoryRepo) CreateJob(_ context.Context, job model.Job) (model.Job, error) {
	r.jobCount++
	job.ID = model.ID(fmt.Sprintf("job-%d", r.jobCount))
	r.job = job
	return job, nil
}

func (r *memoryRepo) UpdateJob(_ context.Context, job model.Job) error {
	r.job = job
	return nil
}

func (r *memoryRepo) UpsertProduct(_ context.Context, product model.Product) (model.Product, error) {
	product.ID = model.ID(product.ExternalID)
	r.products = append(r.products, product)
	return product, nil
}

func (r *memoryRepo) InsertPriceSnapshot(_ context.Context, snapshot model.PriceSnapshot) (model.PriceSnapshot, error) {
	snapshot.ID = model.ID("snap-1")
	return snapshot, nil
}

func (r *memoryRepo) ListProducts(_ context.Context, filter ProductListFilter) ([]model.Product, error) {
	filtered := make([]model.Product, 0, len(r.products))
	for _, product := range r.products {
		if filter.SelfOperatedOnly && product.ShopType != model.ShopTypeSelfOperated {
			continue
		}
		if filter.Category != "" {
			category, _ := product.Attributes["category"].(string)
			if category != filter.Category {
				continue
			}
		}
		if filter.RealOnly {
			mock, _ := product.RawPayload["mock"].(bool)
			if mock {
				continue
			}
		}
		filtered = append(filtered, product)
	}
	return filtered, nil
}

func TestSearchAndStore(t *testing.T) {
	repo := &memoryRepo{}
	service := New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})

	response, err := service.SearchAndStore(context.Background(), SearchRequest{
		Keyword:  "RTX 4060",
		Category: "GPU",
		Limit:    2,
		Persist:  true,
	}, "mock")
	if err != nil {
		t.Fatalf("SearchAndStore() error = %v", err)
	}

	if response.JobID == "" {
		t.Fatal("expected job id")
	}
	if response.PersistedCount != 2 {
		t.Fatalf("expected persisted count 2, got %d", response.PersistedCount)
	}
	if len(repo.products) != 2 {
		t.Fatalf("expected 2 products in repo, got %d", len(repo.products))
	}
}

func TestListProductsFilters(t *testing.T) {
	repo := &memoryRepo{
		products: []model.Product{
			{ID: "gpu-mock", Title: "RTX 4060 官方自营", ShopType: model.ShopTypeSelfOperated, Attributes: map[string]any{"category": "GPU"}, RawPayload: map[string]any{"mock": true}},
			{ID: "gpu-real", Title: "NVIDIA RTX 4060 京东自营", ShopType: model.ShopTypeSelfOperated, Attributes: map[string]any{"category": "GPU"}},
		},
	}
	service := New(repo, jdclient.NewMockClient(), nil)

	products, err := service.ListProducts(context.Background(), ProductListFilter{
		Category:         "GPU",
		Limit:            10,
		SelfOperatedOnly: true,
		RealOnly:         true,
	})
	if err != nil {
		t.Fatalf("ListProducts() error = %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 filtered product, got %d", len(products))
	}
	if products[0].ID != "gpu-real" {
		t.Fatalf("expected gpu-real, got %s", products[0].ID)
	}
}
