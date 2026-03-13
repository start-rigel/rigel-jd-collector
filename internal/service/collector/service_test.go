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
	jobs     []model.Job
	jobCount int
	products []model.Product
}

func (r *memoryRepo) CreateJob(_ context.Context, job model.Job) (model.Job, error) {
	r.jobCount++
	job.ID = model.ID(fmt.Sprintf("job-%d", r.jobCount))
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
	if len(r.jobs) > 0 {
		return r.jobs, nil
	}
	return []model.Job{r.job}, nil
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

func TestRetryJob(t *testing.T) {
	repo := &memoryRepo{
		jobs: []model.Job{{
			ID:     "job-1",
			Status: model.JobSucceeded,
			Payload: map[string]any{
				"keyword":  "RTX 4060",
				"category": "GPU",
				"brand":    "",
				"limit":    float64(2),
				"persist":  true,
			},
		}},
		jobCount: 1,
	}
	service := New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})

	response, err := service.RetryJob(context.Background(), "job-1", "mock")
	if err != nil {
		t.Fatalf("RetryJob() error = %v", err)
	}
	if response.RetriedFromJob != "job-1" {
		t.Fatalf("expected retried_from_job_id job-1, got %s", response.RetriedFromJob)
	}
	if response.JobID == "" || response.JobID == "job-1" {
		t.Fatalf("expected a new job id, got %s", response.JobID)
	}
}

func TestSearchBatchAndStore(t *testing.T) {
	repo := &memoryRepo{}
	service := New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})

	response, err := service.SearchBatchAndStore(context.Background(), []SearchRequest{
		{Keyword: "RTX 4060", Category: "GPU", Limit: 1, Persist: true},
		{Keyword: "Ryzen 5 7500F", Category: "CPU", Limit: 1, Persist: true},
	}, "mock", "mvp_base", false, false)
	if err != nil {
		t.Fatalf("SearchBatchAndStore() error = %v", err)
	}
	if response.TotalJobs != 2 {
		t.Fatalf("expected 2 jobs, got %d", response.TotalJobs)
	}
	if response.SuccessfulJobs != 2 {
		t.Fatalf("expected 2 successful jobs, got %d", response.SuccessfulJobs)
	}
	if response.TotalPersisted != 2 {
		t.Fatalf("expected 2 persisted records, got %d", response.TotalPersisted)
	}
	if len(response.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(response.Results))
	}
}

func TestSearchBatchAndStoreContinueOnError(t *testing.T) {
	repo := &memoryRepo{}
	service := New(repo, errorClient{}, func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})

	response, err := service.SearchBatchAndStore(context.Background(), []SearchRequest{
		{Keyword: "RTX 4060", Category: "GPU", Limit: 1, Persist: true},
		{Keyword: "Ryzen 5 7500F", Category: "CPU", Limit: 1, Persist: true},
	}, "browser", "mvp_base", true, false)
	if err != nil {
		t.Fatalf("SearchBatchAndStore() error = %v", err)
	}
	if response.FailedJobs != 2 {
		t.Fatalf("expected 2 failed jobs, got %d", response.FailedJobs)
	}
	if response.SuccessfulJobs != 0 {
		t.Fatalf("expected 0 successful jobs, got %d", response.SuccessfulJobs)
	}
	if response.Results[0].Error == "" {
		t.Fatal("expected batch result error")
	}
}

func TestSearchBatchAndStoreSkipsExistingRealProducts(t *testing.T) {
	repo := &memoryRepo{
		products: []model.Product{
			{ID: "gpu-1", Title: "RTX 4060 官方自营", ShopType: model.ShopTypeSelfOperated, Attributes: map[string]any{"category": "GPU"}, RawPayload: map[string]any{"browser_collector_mode": "public_search"}},
		},
	}
	service := New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})

	response, err := service.SearchBatchAndStore(context.Background(), []SearchRequest{
		{Keyword: "RTX 4060", Category: "GPU", Limit: 1, Persist: true},
	}, "browser", "mvp_base", true, true)
	if err != nil {
		t.Fatalf("SearchBatchAndStore() error = %v", err)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(response.Results))
	}
	if !response.Results[0].Skipped {
		t.Fatal("expected existing real product to be skipped")
	}
	if response.TotalJobs != 0 {
		t.Fatalf("expected 0 executed jobs, got %d", response.TotalJobs)
	}
}

func TestSearchBatchAndStoreAbortsRemainingRequestsAfterRiskControl(t *testing.T) {
	repo := &memoryRepo{}
	service := New(repo, riskClient{}, func() time.Time {
		return time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	})

	response, err := service.SearchBatchAndStore(context.Background(), []SearchRequest{
		{Keyword: "SN770 1TB", Category: "SSD", Limit: 1, Persist: true},
		{Keyword: "MATX 机箱", Category: "CASE", Limit: 1, Persist: true},
		{Keyword: "AG400", Category: "COOLER", Limit: 1, Persist: true},
	}, "browser", "mvp_base", true, false)
	if err != nil {
		t.Fatalf("SearchBatchAndStore() error = %v", err)
	}
	if response.TotalJobs != 1 {
		t.Fatalf("expected 1 executed job before abort, got %d", response.TotalJobs)
	}
	if response.FailedJobs != 1 {
		t.Fatalf("expected 1 failed job, got %d", response.FailedJobs)
	}
	if response.AbortedJobs != 2 {
		t.Fatalf("expected 2 aborted jobs, got %d", response.AbortedJobs)
	}
	if len(response.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(response.Results))
	}
	if !response.Results[1].Aborted || !response.Results[2].Aborted {
		t.Fatal("expected remaining requests to be marked as aborted")
	}
}

func TestPresetRequestsPrioritizesPSUBeforeSSDUnderRisk(t *testing.T) {
	requests, err := PresetRequests("mvp_base")
	if err != nil {
		t.Fatalf("PresetRequests() error = %v", err)
	}

	psuIndex := -1
	ssdIndex := -1
	for index, req := range requests {
		switch req.Category {
		case "PSU":
			psuIndex = index
		case "SSD":
			ssdIndex = index
		}
	}

	if psuIndex == -1 || ssdIndex == -1 {
		t.Fatalf("expected both PSU and SSD requests, got psu=%d ssd=%d", psuIndex, ssdIndex)
	}
	if psuIndex > ssdIndex {
		t.Fatalf("expected PSU to be attempted before SSD, got psu=%d ssd=%d", psuIndex, ssdIndex)
	}
}

type errorClient struct{}

func (errorClient) SearchProducts(_ context.Context, _ model.SearchQuery) ([]model.Product, error) {
	return nil, fmt.Errorf("upstream failed")
}

type riskClient struct{}

func (riskClient) SearchProducts(_ context.Context, _ model.SearchQuery) ([]model.Product, error) {
	return nil, fmt.Errorf("browser collector returned status 500: {\"error\":\"jd public search was redirected to risk-control\",\"name\":\"RiskControlError\"}")
}
