package collector

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/rigel-labs/rigel-jd-collector/internal/adapter/jdclient"
	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
)

// Repository describes the persistence capabilities required by the collector flow.
type Repository interface {
	CreateJob(ctx context.Context, job model.Job) (model.Job, error)
	UpdateJob(ctx context.Context, job model.Job) error
	ListEnabledKeywordSeeds(ctx context.Context) ([]model.KeywordSeed, error)
	UpsertProduct(ctx context.Context, product model.Product) (model.Product, error)
	InsertPriceSnapshot(ctx context.Context, snapshot model.PriceSnapshot) (model.PriceSnapshot, error)
	EnsurePart(ctx context.Context, part model.Part) (model.Part, error)
	UpsertProductMapping(ctx context.Context, mapping model.ProductPartMapping) error
	UpsertPartMarketSummary(ctx context.Context, summary model.PartMarketSummary) error
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

type ScheduledCollectionRequest struct {
	Persist         bool
	QueryLimit      int
	RequestInterval time.Duration
}

type ScheduledCollectionResult struct {
	JobID             model.ID `json:"job_id"`
	SeedCount         int      `json:"seed_count"`
	SuccessCount      int      `json:"success_count"`
	FailureCount      int      `json:"failure_count"`
	PersistedProducts int      `json:"persisted_products"`
	UpdatedSummaries  int      `json:"updated_summaries"`
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

func (s *Service) RunScheduledCollection(ctx context.Context, req ScheduledCollectionRequest, mode string) (ScheduledCollectionResult, error) {
	if req.QueryLimit <= 0 {
		req.QueryLimit = 3
	}
	if req.RequestInterval < 0 {
		req.RequestInterval = 0
	}
	if !req.Persist {
		req.Persist = true
	}

	seeds, err := s.repo.ListEnabledKeywordSeeds(ctx)
	if err != nil {
		return ScheduledCollectionResult{}, err
	}

	now := s.clock().UTC()
	job, err := s.repo.CreateJob(ctx, model.Job{
		JobType:        model.JobTypeMarketSummary,
		Status:         model.JobQueued,
		SourcePlatform: model.PlatformJD,
		Payload: map[string]any{
			"seed_count":         len(seeds),
			"query_limit":        req.QueryLimit,
			"request_interval":   req.RequestInterval.String(),
			"persist":            req.Persist,
			"collection_trigger": "scheduler",
		},
		ScheduledAt: &now,
	})
	if err != nil {
		return ScheduledCollectionResult{}, err
	}

	startedAt := s.clock().UTC()
	job.Status = model.JobRunning
	job.StartedAt = &startedAt
	if err := s.repo.UpdateJob(ctx, job); err != nil {
		return ScheduledCollectionResult{}, err
	}

	result := ScheduledCollectionResult{JobID: job.ID, SeedCount: len(seeds)}
	var failures []string

	for index, seed := range seeds {
		if index > 0 && req.RequestInterval > 0 {
			if err := sleepWithContext(ctx, req.RequestInterval); err != nil {
				finishJobWithError(ctx, s.repo, job, s.clock().UTC(), err)
				return result, err
			}
		}

		searchResponse, err := s.SearchAndStore(ctx, SearchRequest{
			Keyword:  seed.Keyword,
			Category: string(seed.Category),
			Brand:    seed.Brand,
			Limit:    req.QueryLimit,
			Persist:  true,
		}, mode)
		if err != nil {
			result.FailureCount++
			failures = append(failures, fmt.Sprintf("%s: %v", seed.Keyword, err))
			continue
		}

		result.PersistedProducts += searchResponse.PersistedCount
		if err := s.persistSeedSummary(ctx, seed, searchResponse.Products); err != nil {
			result.FailureCount++
			failures = append(failures, fmt.Sprintf("%s summary: %v", seed.Keyword, err))
			continue
		}
		result.SuccessCount++
		result.UpdatedSummaries++
	}

	finishedAt := s.clock().UTC()
	job.FinishedAt = &finishedAt
	job.Result = map[string]any{
		"seed_count":         result.SeedCount,
		"success_count":      result.SuccessCount,
		"failure_count":      result.FailureCount,
		"persisted_products": result.PersistedProducts,
		"updated_summaries":  result.UpdatedSummaries,
		"request_interval":   req.RequestInterval.String(),
		"query_limit":        req.QueryLimit,
		"failures":           failures,
	}
	if result.SuccessCount == 0 && result.FailureCount > 0 {
		job.Status = model.JobFailed
		job.ErrorMessage = strings.Join(failures, " | ")
	} else {
		job.Status = model.JobSucceeded
		if len(failures) > 0 {
			job.ErrorMessage = strings.Join(failures, " | ")
		}
	}
	if err := s.repo.UpdateJob(ctx, job); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) ListProducts(ctx context.Context, filter ProductListFilter) ([]model.Product, error) {
	return s.repo.ListProducts(ctx, filter)
}

func (s *Service) persistSeedSummary(ctx context.Context, seed model.KeywordSeed, products []model.Product) error {
	if len(products) == 0 {
		return nil
	}
	part, err := s.repo.EnsurePart(ctx, model.Part{
		Category:         seed.Category,
		Brand:            strings.TrimSpace(seed.Brand),
		Model:            seed.CanonicalModel,
		DisplayName:      seed.CanonicalModel,
		NormalizedKey:    normalizedKey(seed.Category, seed.CanonicalModel),
		LifecycleStatus:  "active",
		SourceConfidence: 1,
		AliasKeywords:    buildAliasKeywords(seed),
	})
	if err != nil {
		return err
	}

	for _, product := range products {
		if product.ID == "" {
			continue
		}
		if err := s.repo.UpsertProductMapping(ctx, model.ProductPartMapping{
			ProductID:            product.ID,
			PartID:               part.ID,
			KeywordSeedID:        seed.ID,
			MappingStatus:        model.MappingStatusMapped,
			MatchConfidence:      1,
			MatchedBy:            "keyword_seed",
			CandidateDisplayName: seed.CanonicalModel,
			Reason:               "mapped directly from scheduled keyword seed",
		}); err != nil {
			return err
		}
	}

	summary := summarizeProductsForDay(part.ID, products, s.clock().UTC())
	return s.repo.UpsertPartMarketSummary(ctx, summary)
}

func summarizeProductsForDay(partID model.ID, products []model.Product, collectedAt time.Time) model.PartMarketSummary {
	prices := make([]float64, 0, len(products))
	latestPrice := 0.0
	latestSeen := time.Time{}
	for index, product := range products {
		prices = append(prices, product.Price)
		seenAt := product.LastSeenAt
		if seenAt.IsZero() {
			seenAt = product.UpdatedAt
		}
		if seenAt.IsZero() {
			seenAt = product.CreatedAt
		}
		if seenAt.IsZero() {
			seenAt = collectedAt.Add(time.Duration(index) * time.Millisecond)
		}
		if latestSeen.IsZero() || seenAt.After(latestSeen) {
			latestSeen = seenAt
			latestPrice = product.Price
		}
	}
	snapshotDate := time.Date(collectedAt.Year(), collectedAt.Month(), collectedAt.Day(), 0, 0, 0, 0, time.UTC)
	collectedAtCopy := collectedAt
	return model.PartMarketSummary{
		PartID:          partID,
		SourcePlatform:  model.PlatformJD,
		SnapshotDate:    snapshotDate,
		LatestPrice:     latestPrice,
		MinPrice:        minPrice(prices),
		MaxPrice:        maxPrice(prices),
		MedianPrice:     medianPrice(prices),
		P25Price:        percentilePrice(prices, 0.25),
		P75Price:        percentilePrice(prices, 0.75),
		SampleCount:     len(products),
		LastCollectedAt: &collectedAtCopy,
	}
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

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func buildAliasKeywords(seed model.KeywordSeed) []string {
	seen := map[string]struct{}{}
	values := []string{seed.Keyword, seed.CanonicalModel}
	values = append(values, seed.Aliases...)
	aliases := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		aliases = append(aliases, trimmed)
	}
	return aliases
}

func normalizedKey(category model.PartCategory, canonicalModel string) string {
	prefix := strings.ToLower(string(category))
	builder := strings.Builder{}
	lastDash := false
	for _, r := range strings.ToLower(canonicalModel) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if lastDash {
			continue
		}
		builder.WriteRune('-')
		lastDash = true
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return prefix
	}
	return prefix + "-" + slug
}

func minPrice(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}
	min := prices[0]
	for _, price := range prices[1:] {
		if price < min {
			min = price
		}
	}
	return roundPrice(min)
}

func maxPrice(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}
	max := prices[0]
	for _, price := range prices[1:] {
		if price > max {
			max = price
		}
	}
	return roundPrice(max)
}

func medianPrice(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}
	sorted := append([]float64(nil), prices...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return roundPrice(sorted[middle])
	}
	return roundPrice((sorted[middle-1] + sorted[middle]) / 2)
}

func percentilePrice(prices []float64, percentile float64) float64 {
	if len(prices) == 0 {
		return 0
	}
	if percentile <= 0 {
		return minPrice(prices)
	}
	if percentile >= 1 {
		return maxPrice(prices)
	}
	sorted := append([]float64(nil), prices...)
	sort.Float64s(sorted)
	index := percentile * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	if lower == upper {
		return roundPrice(sorted[lower])
	}
	weight := index - float64(lower)
	return roundPrice(sorted[lower] + (sorted[upper]-sorted[lower])*weight)
}

func roundPrice(value float64) float64 {
	return math.Round(value*100) / 100
}
