package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rigel-labs/rigel-jd-collector/internal/adapter/jdclient"
	"github.com/rigel-labs/rigel-jd-collector/internal/config"
	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
	collectorservice "github.com/rigel-labs/rigel-jd-collector/internal/service/collector"
)

type memoryRepo struct {
	products []model.Product
}

func (r *memoryRepo) CreateJob(_ context.Context, job model.Job) (model.Job, error) {
	job.ID = "job-1"
	return job, nil
}
func (r *memoryRepo) UpdateJob(_ context.Context, _ model.Job) error { return nil }
func (r *memoryRepo) UpsertProduct(_ context.Context, product model.Product) (model.Product, error) {
	if product.ID == "" {
		product.ID = model.ID(product.ExternalID)
	}
	r.products = append(r.products, product)
	return product, nil
}
func (r *memoryRepo) InsertPriceSnapshot(_ context.Context, snapshot model.PriceSnapshot) (model.PriceSnapshot, error) {
	snapshot.ID = "snapshot-1"
	return snapshot, nil
}
func (r *memoryRepo) ListProducts(_ context.Context, _ collectorservice.ProductListFilter) ([]model.Product, error) {
	return r.products, nil
}

func TestHealthz(t *testing.T) {
	repo := &memoryRepo{}
	collector := collectorservice.New(repo, jdclient.NewMockClient(), nil)
	application := New(config.Config{ServiceName: "test-service", JDCollectorMode: "mock"}, collector)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestCollectSearch(t *testing.T) {
	repo := &memoryRepo{}
	collector := collectorservice.New(repo, jdclient.NewMockClient(), nil)
	application := New(config.Config{ServiceName: "test-service", JDCollectorMode: "mock"}, collector)
	body := []byte(`{"keyword":"RTX 4060","category":"GPU","limit":2,"persist":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collect/search", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListProducts(t *testing.T) {
	repo := &memoryRepo{products: []model.Product{{ID: "gpu-1", Title: "RTX 4060 官方自营", Price: 1999}}}
	collector := collectorservice.New(repo, jdclient.NewMockClient(), nil)
	application := New(config.Config{ServiceName: "test-service", JDCollectorMode: "mock"}, collector)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?keyword=4060&limit=10", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
