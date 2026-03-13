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
	job      model.Job
	jobs     []model.Job
	jobCount int
	products []model.Product
}

func (r *memoryRepo) CreateJob(_ context.Context, job model.Job) (model.Job, error) {
	r.jobCount++
	job.ID = model.ID(time.Date(2026, 3, 12, 8, 0, r.jobCount, 0, time.UTC).Format("job-150405"))
	r.job = job
	r.jobs = append([]model.Job{job}, r.jobs...)
	return job, nil
}

func (r *memoryRepo) UpdateJob(_ context.Context, job model.Job) error {
	r.job = job
	for index := range r.jobs {
		if r.jobs[index].ID == job.ID {
			r.jobs[index] = job
			return nil
		}
	}
	return nil
}

func (r *memoryRepo) GetJob(_ context.Context, jobID model.ID) (model.Job, error) {
	for _, job := range r.jobs {
		if job.ID == jobID {
			return job, nil
		}
	}
	return r.job, nil
}

func (r *memoryRepo) ListJobs(_ context.Context, _ int) ([]model.Job, error) {
	if len(r.jobs) == 0 && r.job.ID != "" {
		return []model.Job{r.job}, nil
	}
	return r.jobs, nil
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

func (r *memoryRepo) ListProducts(_ context.Context, filter collectorservice.ProductListFilter) ([]model.Product, error) {
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

func TestHealthz(t *testing.T) {
	repo := &memoryRepo{}
	collector := collectorservice.New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})
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
	collector := collectorservice.New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})
	application := New(config.Config{ServiceName: "test-service", JDCollectorMode: "mock"}, collector)

	body := []byte(`{"keyword":"RTX 4060","category":"GPU","limit":2}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collect/search", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(repo.products) != 2 {
		t.Fatalf("expected 2 persisted products, got %d", len(repo.products))
	}
}

func TestListJobs(t *testing.T) {
	repo := &memoryRepo{
		jobs: []model.Job{{ID: "job-1", JobType: model.JobTypeJDCollect, Status: model.JobSucceeded}},
	}
	collector := collectorservice.New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})
	application := New(config.Config{ServiceName: "test-service", JDCollectorMode: "mock"}, collector)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?limit=5", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRetryJob(t *testing.T) {
	repo := &memoryRepo{
		jobs: []model.Job{{
			ID:     "job-1",
			Status: model.JobSucceeded,
			Payload: map[string]any{
				"keyword":  "RTX 4060",
				"category": "GPU",
				"limit":    float64(2),
				"persist":  true,
			},
		}},
		jobCount: 1,
	}
	collector := collectorservice.New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})
	application := New(config.Config{ServiceName: "test-service", JDCollectorMode: "mock"}, collector)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/job-1/retry", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListProductsWithFilters(t *testing.T) {
	repo := &memoryRepo{
		products: []model.Product{
			{ID: "1", Title: "RTX 4060 官方自营", ShopType: model.ShopTypeSelfOperated},
			{ID: "2", Title: "RTX 4060 第三方", ShopType: model.ShopTypeMarketplace},
		},
	}
	collector := collectorservice.New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})
	application := New(config.Config{ServiceName: "test-service", JDCollectorMode: "mock"}, collector)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?keyword=RTX%204060&self_operated_only=true", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"count":1`)) {
		t.Fatalf("expected filtered product count 1, got body %s", rec.Body.String())
	}
}

func TestCollectBatchPreset(t *testing.T) {
	repo := &memoryRepo{}
	collector := collectorservice.New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})
	application := New(config.Config{ServiceName: "test-service", JDCollectorMode: "mock"}, collector)

	body := []byte(`{"preset":"mvp_base"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collect/batch", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"total_jobs":8`)) {
		t.Fatalf("expected batch response to include 8 jobs, got body %s", rec.Body.String())
	}
}
