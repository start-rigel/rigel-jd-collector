package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rigel-labs/rigel-jd-collector/internal/adapter/jdclient"
	"github.com/rigel-labs/rigel-jd-collector/internal/config"
	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
	collectorservice "github.com/rigel-labs/rigel-jd-collector/internal/service/collector"
)

type memoryRepo struct {
	products  []model.Product
	seeds     []model.KeywordSeed
	mappings  []model.ProductPartMapping
	summaries []model.PartMarketSummary
	schedule  *model.CollectorScheduleConfig
}

func (r *memoryRepo) CreateJob(_ context.Context, job model.Job) (model.Job, error) {
	job.ID = "job-1"
	return job, nil
}
func (r *memoryRepo) UpdateJob(_ context.Context, _ model.Job) error { return nil }
func (r *memoryRepo) ListEnabledKeywordSeeds(_ context.Context) ([]model.KeywordSeed, error) {
	return r.seeds, nil
}
func (r *memoryRepo) GetCollectorScheduleConfig(_ context.Context, _ string) (model.CollectorScheduleConfig, bool, error) {
	if r.schedule == nil {
		return model.CollectorScheduleConfig{}, false, nil
	}
	return *r.schedule, true, nil
}
func (r *memoryRepo) UpsertCollectorScheduleConfig(_ context.Context, cfg model.CollectorScheduleConfig) (model.CollectorScheduleConfig, error) {
	cfg.ID = "schedule-1"
	r.schedule = &cfg
	return cfg, nil
}
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
func (r *memoryRepo) EnsurePart(_ context.Context, part model.Part) (model.Part, error) {
	part.ID = model.ID(part.NormalizedKey)
	return part, nil
}
func (r *memoryRepo) UpsertProductMapping(_ context.Context, mapping model.ProductPartMapping) error {
	r.mappings = append(r.mappings, mapping)
	return nil
}
func (r *memoryRepo) UpsertPartMarketSummary(_ context.Context, summary model.PartMarketSummary) error {
	r.summaries = append(r.summaries, summary)
	return nil
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

func TestScheduleConfig(t *testing.T) {
	repo := &memoryRepo{}
	collector := collectorservice.New(repo, jdclient.NewMockClient(), nil)
	application := New(config.Config{ServiceName: "test-service", JDCollectorMode: "mock"}, collector)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/schedule", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := []byte(`{"enabled":true,"schedule_time":"03:00","request_interval_seconds":2,"query_limit":3}`)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/schedule", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
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
