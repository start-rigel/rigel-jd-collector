package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/rigel-labs/rigel-jd-collector/internal/adapter/jdclient"
	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
)

// Repository describes the persistence capabilities required by the collector flow.
type Repository interface {
	CreateJob(ctx context.Context, job model.Job) (model.Job, error)
	UpdateJob(ctx context.Context, job model.Job) error
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
		Mode:           mode,
		Persisted:      req.Persist,
		PersistedCount: persistedCount,
		Products:       products,
	}, nil
}

func (s *Service) ListProducts(ctx context.Context, filter ProductListFilter) ([]model.Product, error) {
	return s.repo.ListProducts(ctx, filter)
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
