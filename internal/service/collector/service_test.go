package collector

import (
	"context"
	"testing"
	"time"

	"github.com/rigel-labs/rigel-jd-collector/internal/adapter/jdclient"
	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
)

type scheduleRepo struct {
	jobs      []model.Job
	products  []model.Product
	seeds     []model.KeywordSeed
	mappings  []model.ProductPartMapping
	summaries []model.PartMarketSummary
	schedule  *model.CollectorScheduleConfig
}

func (r *scheduleRepo) CreateJob(_ context.Context, job model.Job) (model.Job, error) {
	job.ID = model.ID("job-" + time.Now().Format("150405.000"))
	r.jobs = append(r.jobs, job)
	return job, nil
}
func (r *scheduleRepo) UpdateJob(_ context.Context, job model.Job) error {
	r.jobs = append(r.jobs, job)
	return nil
}
func (r *scheduleRepo) ListEnabledKeywordSeeds(_ context.Context) ([]model.KeywordSeed, error) {
	return r.seeds, nil
}
func (r *scheduleRepo) GetCollectorScheduleConfig(_ context.Context, serviceName string) (model.CollectorScheduleConfig, bool, error) {
	if r.schedule == nil || r.schedule.ServiceName != serviceName {
		return model.CollectorScheduleConfig{}, false, nil
	}
	return *r.schedule, true, nil
}
func (r *scheduleRepo) UpsertCollectorScheduleConfig(_ context.Context, cfg model.CollectorScheduleConfig) (model.CollectorScheduleConfig, error) {
	now := time.Now().UTC()
	if cfg.ID == "" {
		cfg.ID = model.ID("schedule-1")
		cfg.CreatedAt = now
	}
	cfg.UpdatedAt = now
	cloned := cfg
	r.schedule = &cloned
	return cfg, nil
}
func (r *scheduleRepo) UpsertProduct(_ context.Context, product model.Product) (model.Product, error) {
	if product.ID == "" {
		product.ID = model.ID(product.ExternalID)
	}
	r.products = append(r.products, product)
	return product, nil
}
func (r *scheduleRepo) InsertPriceSnapshot(_ context.Context, snapshot model.PriceSnapshot) (model.PriceSnapshot, error) {
	snapshot.ID = model.ID("snapshot")
	return snapshot, nil
}
func (r *scheduleRepo) EnsurePart(_ context.Context, part model.Part) (model.Part, error) {
	part.ID = model.ID(part.NormalizedKey)
	return part, nil
}
func (r *scheduleRepo) UpsertProductMapping(_ context.Context, mapping model.ProductPartMapping) error {
	r.mappings = append(r.mappings, mapping)
	return nil
}
func (r *scheduleRepo) UpsertPartMarketSummary(_ context.Context, summary model.PartMarketSummary) error {
	r.summaries = append(r.summaries, summary)
	return nil
}
func (r *scheduleRepo) ListProducts(_ context.Context, _ ProductListFilter) ([]model.Product, error) {
	return r.products, nil
}

func TestRunScheduledCollection(t *testing.T) {
	repo := &scheduleRepo{seeds: []model.KeywordSeed{{ID: "seed-1", Category: model.CategoryGPU, Keyword: "RTX 4060", CanonicalModel: "RTX 4060", Brand: "NVIDIA"}}}
	service := New(repo, jdclient.NewMockClient(), func() time.Time {
		return time.Date(2026, 3, 19, 3, 0, 0, 0, time.UTC)
	})

	result, err := service.RunScheduledCollection(context.Background(), ScheduledCollectionRequest{Persist: true, QueryLimit: 2, RequestInterval: 0}, "mock")
	if err != nil {
		t.Fatalf("RunScheduledCollection() error = %v", err)
	}
	if result.SeedCount != 1 || result.SuccessCount != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(repo.summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(repo.summaries))
	}
	if repo.summaries[0].SnapshotDate.Format("2006-01-02") != "2026-03-19" {
		t.Fatalf("unexpected snapshot date: %s", repo.summaries[0].SnapshotDate)
	}
	if len(repo.mappings) == 0 {
		t.Fatal("expected product mappings to be written")
	}
}

func TestNextScheduledTime(t *testing.T) {
	now := time.Date(2026, 3, 19, 3, 10, 0, 0, time.FixedZone("CST", 8*3600))
	next, err := nextScheduledTime(now, "03:00")
	if err != nil {
		t.Fatalf("nextScheduledTime() error = %v", err)
	}
	if next.Day() != 20 || next.Hour() != 3 || next.Minute() != 0 {
		t.Fatalf("unexpected next run: %s", next)
	}
}

func TestCollectorScheduleConfigCRUD(t *testing.T) {
	repo := &scheduleRepo{}
	service := New(repo, jdclient.NewMockClient(), time.Now)

	if _, exists, err := service.GetCollectorScheduleConfig(context.Background(), "rigel-jd-collector"); err != nil {
		t.Fatalf("GetCollectorScheduleConfig() error = %v", err)
	} else if exists {
		t.Fatal("expected empty schedule config")
	}

	cfg, err := service.UpsertCollectorScheduleConfig(context.Background(), "rigel-jd-collector", CollectorScheduleUpsertRequest{
		Enabled:                true,
		ScheduleTime:           "03:30",
		RequestIntervalSeconds: 5,
		QueryLimit:             8,
	})
	if err != nil {
		t.Fatalf("UpsertCollectorScheduleConfig() error = %v", err)
	}
	if cfg.ServiceName != "rigel-jd-collector" || cfg.ScheduleTime != "03:30" || cfg.QueryLimit != 8 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
