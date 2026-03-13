package collector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rigel-labs/rigel-jd-collector/internal/adapter/jdclient"
	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
)

// Repository describes the persistence capabilities required by the collector flow.
type Repository interface {
	CreateJob(ctx context.Context, job model.Job) (model.Job, error)
	UpdateJob(ctx context.Context, job model.Job) error
	GetJob(ctx context.Context, jobID model.ID) (model.Job, error)
	ListJobs(ctx context.Context, limit int) ([]model.Job, error)
	UpsertProduct(ctx context.Context, product model.Product) (model.Product, error)
	InsertPriceSnapshot(ctx context.Context, snapshot model.PriceSnapshot) (model.PriceSnapshot, error)
	ListProducts(ctx context.Context, filter ProductListFilter) ([]model.Product, error)
}

// Service orchestrates the JD search adapter and persistence logic.
type Service struct {
	repo   Repository
	client jdclient.Client
	clock  func() time.Time
}

const batchRequestDelay = 3 * time.Second

// SearchRequest describes a minimum viable collection request.
type SearchRequest struct {
	Keyword  string
	Category string
	Brand    string
	Limit    int
	Persist  bool
}

// SearchResponse contains the search result and persistence outcome.
type SearchResponse struct {
	JobID          model.ID        `json:"job_id"`
	RetriedFromJob model.ID        `json:"retried_from_job_id,omitempty"`
	Mode           string          `json:"mode"`
	Persisted      bool            `json:"persisted"`
	PersistedCount int             `json:"persisted_count"`
	Products       []model.Product `json:"products"`
}

type ProductListFilter struct {
	Keyword          string
	Category         string
	Limit            int
	ShopType         model.ShopType
	RealOnly         bool
	SelfOperatedOnly bool
}

type BatchResponse struct {
	Preset         string            `json:"preset,omitempty"`
	Mode           string            `json:"mode"`
	TotalJobs      int               `json:"total_jobs"`
	SuccessfulJobs int               `json:"successful_jobs"`
	FailedJobs     int               `json:"failed_jobs"`
	SkippedJobs    int               `json:"skipped_jobs"`
	AbortedJobs    int               `json:"aborted_jobs"`
	TotalPersisted int               `json:"total_persisted"`
	Results        []BatchItemResult `json:"results"`
}

type BatchItemResult struct {
	Keyword  string          `json:"keyword"`
	Category string          `json:"category"`
	Brand    string          `json:"brand,omitempty"`
	Skipped  bool            `json:"skipped,omitempty"`
	Aborted  bool            `json:"aborted,omitempty"`
	Note     string          `json:"note,omitempty"`
	Response *SearchResponse `json:"response,omitempty"`
	Error    string          `json:"error,omitempty"`
}

func New(repo Repository, client jdclient.Client, clock func() time.Time) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{repo: repo, client: client, clock: clock}
}

func (s *Service) SearchAndStore(ctx context.Context, req SearchRequest, mode string) (SearchResponse, error) {
	if req.Keyword == "" {
		return SearchResponse{}, fmt.Errorf("keyword must not be empty")
	}
	if req.Limit <= 0 {
		req.Limit = 3
	}
	return s.executeSearch(ctx, req, mode, "")
}

func (s *Service) RetryJob(ctx context.Context, jobID model.ID, mode string) (SearchResponse, error) {
	if jobID == "" {
		return SearchResponse{}, fmt.Errorf("job id is required")
	}
	job, err := s.repo.GetJob(ctx, jobID)
	if err != nil {
		return SearchResponse{}, err
	}
	req, err := searchRequestFromPayload(job.Payload)
	if err != nil {
		return SearchResponse{}, err
	}
	return s.executeSearch(ctx, req, mode, jobID)
}

func (s *Service) SearchBatchAndStore(ctx context.Context, requests []SearchRequest, mode, preset string, continueOnError, skipExistingReal bool) (BatchResponse, error) {
	if len(requests) == 0 {
		return BatchResponse{}, fmt.Errorf("at least one search request is required")
	}

	response := BatchResponse{
		Preset:  preset,
		Mode:    mode,
		Results: make([]BatchItemResult, 0, len(requests)),
	}
	abortedReason := ""
	for index, req := range requests {
		item := BatchItemResult{
			Keyword:  req.Keyword,
			Category: req.Category,
			Brand:    req.Brand,
		}
		if abortedReason != "" {
			item.Aborted = true
			item.Note = abortedReason
			response.Results = append(response.Results, item)
			response.AbortedJobs++
			continue
		}
		if skipExistingReal {
			exists, err := s.hasExistingRealProduct(ctx, req)
			if err != nil {
				return response, err
			}
			if exists {
				item.Skipped = true
				item.Note = "skipped because real self-operated records already exist"
				response.Results = append(response.Results, item)
				response.SkippedJobs++
				continue
			}
		}

		result, err := s.SearchAndStore(ctx, req, mode)
		if err != nil {
			item.Error = err.Error()
			response.Results = append(response.Results, item)
			response.TotalJobs++
			response.FailedJobs++
			if shouldAbortBatchAfterError(err) {
				abortedReason = "aborted remaining requests after JD risk control was detected"
			}
			if !continueOnError {
				return response, err
			}
		} else {
			item.Response = &result
			response.Results = append(response.Results, item)
			response.TotalJobs++
			response.SuccessfulJobs++
			response.TotalPersisted += result.PersistedCount
		}
		if index < len(requests)-1 {
			if err := sleepWithContext(ctx, batchRequestDelay); err != nil {
				return response, err
			}
		}
	}
	return response, nil
}

func shouldAbortBatchAfterError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "riskcontrolerror") || strings.Contains(message, "risk-control")
}

func (s *Service) hasExistingRealProduct(ctx context.Context, req SearchRequest) (bool, error) {
	products, err := s.repo.ListProducts(ctx, ProductListFilter{
		Category:         req.Category,
		Limit:            1,
		RealOnly:         true,
		SelfOperatedOnly: true,
	})
	if err != nil {
		return false, err
	}
	return len(products) > 0, nil
}

func (s *Service) executeSearch(ctx context.Context, req SearchRequest, mode string, retriedFromJobID model.ID) (SearchResponse, error) {
	if req.Keyword == "" {
		return SearchResponse{}, fmt.Errorf("keyword must not be empty")
	}
	if req.Limit <= 0 {
		req.Limit = 3
	}

	now := s.clock().UTC()
	job, err := s.repo.CreateJob(ctx, model.Job{
		JobType:        model.JobTypeJDCollect,
		Status:         model.JobQueued,
		SourcePlatform: model.PlatformJD,
		Payload: map[string]any{
			"keyword":  req.Keyword,
			"category": req.Category,
			"brand":    req.Brand,
			"limit":    req.Limit,
			"persist":  req.Persist,
		},
		ScheduledAt: &now,
	})
	if err != nil {
		return SearchResponse{}, err
	}

	startedAt := s.clock().UTC()
	job.Status = model.JobRunning
	job.StartedAt = &startedAt
	if err := s.repo.UpdateJob(ctx, job); err != nil {
		return SearchResponse{}, err
	}

	products, err := s.client.SearchProducts(ctx, model.SearchQuery{
		Keyword:  req.Keyword,
		Category: req.Category,
		Brand:    req.Brand,
		Limit:    req.Limit,
	})
	if err != nil {
		finishJobWithError(ctx, s.repo, job, s.clock().UTC(), err)
		return SearchResponse{}, err
	}

	persistedCount := 0
	if req.Persist {
		for index, product := range products {
			persistedProduct, err := s.repo.UpsertProduct(ctx, product)
			if err != nil {
				finishJobWithError(ctx, s.repo, job, s.clock().UTC(), err)
				return SearchResponse{}, err
			}

			products[index] = persistedProduct
			if _, err := s.repo.InsertPriceSnapshot(ctx, model.PriceSnapshot{
				ProductID:      persistedProduct.ID,
				SourcePlatform: model.PlatformJD,
				Price:          persistedProduct.Price,
				InStock:        persistedProduct.Availability != "out_of_stock",
				CapturedAt:     s.clock().UTC(),
				Metadata: map[string]any{
					"mode":     mode,
					"keyword":  req.Keyword,
					"category": req.Category,
				},
			}); err != nil {
				finishJobWithError(ctx, s.repo, job, s.clock().UTC(), err)
				return SearchResponse{}, err
			}
			persistedCount++
		}
	}

	finishedAt := s.clock().UTC()
	job.Status = model.JobSucceeded
	job.FinishedAt = &finishedAt
	job.Result = map[string]any{
		"mode":            mode,
		"product_count":   len(products),
		"persisted_count": persistedCount,
		"keyword":         req.Keyword,
	}
	if err := s.repo.UpdateJob(ctx, job); err != nil {
		return SearchResponse{}, err
	}

	return SearchResponse{
		JobID:          job.ID,
		RetriedFromJob: retriedFromJobID,
		Mode:           mode,
		Persisted:      req.Persist,
		PersistedCount: persistedCount,
		Products:       products,
	}, nil
}

func (s *Service) ListProducts(ctx context.Context, filter ProductListFilter) ([]model.Product, error) {
	return s.repo.ListProducts(ctx, filter)
}

func (s *Service) GetJob(ctx context.Context, jobID model.ID) (model.Job, error) {
	return s.repo.GetJob(ctx, jobID)
}

func (s *Service) ListJobs(ctx context.Context, limit int) ([]model.Job, error) {
	return s.repo.ListJobs(ctx, limit)
}

func finishJobWithError(ctx context.Context, repo Repository, job model.Job, finishedAt time.Time, originalErr error) {
	job.Status = model.JobFailed
	job.FinishedAt = &finishedAt
	job.ErrorMessage = originalErr.Error()
	job.Result = map[string]any{
		"error": originalErr.Error(),
	}
	_ = repo.UpdateJob(ctx, job)
}

func searchRequestFromPayload(payload map[string]any) (SearchRequest, error) {
	req := SearchRequest{
		Keyword:  stringFromPayload(payload, "keyword"),
		Category: stringFromPayload(payload, "category"),
		Brand:    stringFromPayload(payload, "brand"),
		Limit:    intFromPayload(payload, "limit"),
		Persist:  boolFromPayload(payload, "persist", true),
	}
	if req.Keyword == "" {
		return SearchRequest{}, fmt.Errorf("job payload does not include keyword")
	}
	if req.Limit <= 0 {
		req.Limit = 3
	}
	return req, nil
}

func stringFromPayload(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func intFromPayload(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	value, ok := payload[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func boolFromPayload(payload map[string]any, key string, fallback bool) bool {
	if payload == nil {
		return fallback
	}
	value, ok := payload[key]
	if !ok {
		return fallback
	}
	typed, ok := value.(bool)
	if !ok {
		return fallback
	}
	return typed
}

func PresetRequests(name string) ([]SearchRequest, error) {
	switch name {
	case "mvp_base":
		return []SearchRequest{
			{Keyword: "Ryzen 5 7500F", Category: "CPU", Limit: 3, Persist: true},
			{Keyword: "B650M", Category: "MB", Limit: 3, Persist: true},
			{Keyword: "RTX 4060", Category: "GPU", Limit: 3, Persist: true},
			{Keyword: "DDR5 6000 32GB", Category: "RAM", Limit: 3, Persist: true},
			{Keyword: "650W 金牌 电源", Category: "PSU", Limit: 3, Persist: true},
			{Keyword: "MATX 机箱", Category: "CASE", Limit: 3, Persist: true},
			{Keyword: "AG400", Category: "COOLER", Limit: 3, Persist: true},
			{Keyword: "SN770 1TB", Category: "SSD", Limit: 3, Persist: true},
		}, nil
	default:
		return nil, fmt.Errorf("unknown preset %q", name)
	}
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
