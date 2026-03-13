package model

import "time"

type ID string

type SourcePlatform string

type ShopType string

type JobStatus string

type JobType string

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
	JobTypeJDCollect JobType   = "jd_collect"
	JobQueued        JobStatus = "queued"
	JobRunning       JobStatus = "running"
	JobSucceeded     JobStatus = "succeeded"
	JobFailed        JobStatus = "failed"
)

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
