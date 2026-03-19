package model

import "time"

type ID string

type SourcePlatform string

type ShopType string

type JobStatus string

type JobType string

type PartCategory string

type MappingStatus string

const (
	PlatformJD SourcePlatform = "jd"
)

const (
	ShopTypeSelfOperated ShopType = "self_operated"
	ShopTypeFlagship     ShopType = "flagship"
	ShopTypeAuthorized   ShopType = "authorized"
	ShopTypeMarketplace  ShopType = "marketplace"
	ShopTypeUnknown      ShopType = "unknown"
)

const (
	CategoryCPU    PartCategory = "CPU"
	CategoryMB     PartCategory = "MB"
	CategoryGPU    PartCategory = "GPU"
	CategoryRAM    PartCategory = "RAM"
	CategorySSD    PartCategory = "SSD"
	CategoryHDD    PartCategory = "HDD"
	CategoryPSU    PartCategory = "PSU"
	CategoryCase   PartCategory = "CASE"
	CategoryCooler PartCategory = "COOLER"
)

const (
	JobTypeJDCollect     JobType   = "jd_collect"
	JobTypeMarketSummary JobType   = "market_summary"
	JobQueued            JobStatus = "queued"
	JobRunning           JobStatus = "running"
	JobSucceeded         JobStatus = "succeeded"
	JobFailed            JobStatus = "failed"
)

const (
	MappingStatusPending      MappingStatus = "pending"
	MappingStatusMapped       MappingStatus = "mapped"
	MappingStatusRejected     MappingStatus = "rejected"
	MappingStatusManualReview MappingStatus = "manual_review"
)

// KeywordSeed is the configured search seed maintained in the admin UI.
type KeywordSeed struct {
	ID             ID           `json:"id"`
	Category       PartCategory `json:"category"`
	Keyword        string       `json:"keyword"`
	CanonicalModel string       `json:"canonical_model"`
	Brand          string       `json:"brand"`
	Aliases        []string     `json:"aliases,omitempty"`
	Priority       int          `json:"priority"`
	Enabled        bool         `json:"enabled"`
	Notes          string       `json:"notes,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// Part is the canonical hardware entry tied to a keyword seed.
type Part struct {
	ID               ID           `json:"id"`
	Category         PartCategory `json:"category"`
	Brand            string       `json:"brand"`
	Series           string       `json:"series,omitempty"`
	Model            string       `json:"model"`
	DisplayName      string       `json:"display_name"`
	NormalizedKey    string       `json:"normalized_key"`
	Generation       string       `json:"generation,omitempty"`
	MSRP             float64      `json:"msrp,omitempty"`
	ReleaseYear      int          `json:"release_year,omitempty"`
	LifecycleStatus  string       `json:"lifecycle_status"`
	SourceConfidence float64      `json:"source_confidence"`
	AliasKeywords    []string     `json:"alias_keywords,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

// Product is the raw JD product record captured by the collector.
type Product struct {
	ID             ID             `json:"id"`
	SourcePlatform SourcePlatform `json:"source_platform"`
	ExternalID     string         `json:"external_id"`
	SKUID          string         `json:"sku_id"`
	Title          string         `json:"title"`
	Subtitle       string         `json:"subtitle"`
	URL            string         `json:"url"`
	ImageURL       string         `json:"image_url"`
	ShopName       string         `json:"shop_name"`
	ShopType       ShopType       `json:"shop_type"`
	SellerName     string         `json:"seller_name"`
	Region         string         `json:"region"`
	Price          float64        `json:"price"`
	Currency       string         `json:"currency"`
	Availability   string         `json:"availability"`
	Attributes     map[string]any `json:"attributes,omitempty"`
	RawPayload     map[string]any `json:"raw_payload,omitempty"`
	FirstSeenAt    time.Time      `json:"first_seen_at"`
	LastSeenAt     time.Time      `json:"last_seen_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// PriceSnapshot is append-only and should be persisted for historical tracking.
type PriceSnapshot struct {
	ID             ID             `json:"id"`
	ProductID      ID             `json:"product_id"`
	SourcePlatform SourcePlatform `json:"source_platform"`
	Price          float64        `json:"price"`
	InStock        bool           `json:"in_stock"`
	CapturedAt     time.Time      `json:"captured_at"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// ProductPartMapping links a raw product to a canonical part entry.
type ProductPartMapping struct {
	ID                   ID            `json:"id"`
	ProductID            ID            `json:"product_id"`
	PartID               ID            `json:"part_id"`
	KeywordSeedID        ID            `json:"keyword_seed_id"`
	MappingStatus        MappingStatus `json:"mapping_status"`
	MatchConfidence      float64       `json:"match_confidence"`
	MatchedBy            string        `json:"matched_by"`
	CandidateDisplayName string        `json:"candidate_display_name,omitempty"`
	Reason               string        `json:"reason,omitempty"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

// PartMarketSummary stores one model-level daily price snapshot per source.
type PartMarketSummary struct {
	ID              ID             `json:"id"`
	PartID          ID             `json:"part_id"`
	SourcePlatform  SourcePlatform `json:"source_platform"`
	SnapshotDate    time.Time      `json:"snapshot_date"`
	LatestPrice     float64        `json:"latest_price"`
	MinPrice        float64        `json:"min_price"`
	MaxPrice        float64        `json:"max_price"`
	MedianPrice     float64        `json:"median_price"`
	P25Price        float64        `json:"p25_price"`
	P75Price        float64        `json:"p75_price"`
	SampleCount     int            `json:"sample_count"`
	LastCollectedAt *time.Time     `json:"last_collected_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// SearchQuery is the adapter input for JD search clients.
type SearchQuery struct {
	Keyword  string
	Category string
	Brand    string
	Limit    int
}

// Job records each collector execution attempt.
type Job struct {
	ID             ID             `json:"id"`
	JobType        JobType        `json:"job_type"`
	Status         JobStatus      `json:"status"`
	SourcePlatform SourcePlatform `json:"source_platform"`
	Payload        map[string]any `json:"payload,omitempty"`
	Result         map[string]any `json:"result,omitempty"`
	ScheduledAt    *time.Time     `json:"scheduled_at,omitempty"`
	StartedAt      *time.Time     `json:"started_at,omitempty"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
	RetryCount     int            `json:"retry_count"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}
