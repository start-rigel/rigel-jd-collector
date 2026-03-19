package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
	collector "github.com/rigel-labs/rigel-jd-collector/internal/service/collector"
)

// Repository persists collected JD data into PostgreSQL.
type Repository struct {
	db *sql.DB
}

func New(ctx context.Context, dsn string) (*Repository, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Repository{db: db}, nil
}

func (r *Repository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Repository) CreateJob(ctx context.Context, job model.Job) (model.Job, error) {
	payload, err := marshalJSON(job.Payload)
	if err != nil {
		return model.Job{}, err
	}
	result, err := marshalJSON(job.Result)
	if err != nil {
		return model.Job{}, err
	}

	query := `
INSERT INTO rigel_jobs (job_type, status, source_platform, payload, result, scheduled_at, started_at, finished_at, retry_count, error_message)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, created_at, updated_at`

	row := r.db.QueryRowContext(ctx, query,
		job.JobType,
		job.Status,
		nullableString(string(job.SourcePlatform)),
		payload,
		result,
		job.ScheduledAt,
		job.StartedAt,
		job.FinishedAt,
		job.RetryCount,
		nullableString(job.ErrorMessage),
	)

	var id string
	if err := row.Scan(&id, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return model.Job{}, fmt.Errorf("insert job: %w", err)
	}
	job.ID = model.ID(id)
	return job, nil
}

func (r *Repository) UpdateJob(ctx context.Context, job model.Job) error {
	payload, err := marshalJSON(job.Payload)
	if err != nil {
		return err
	}
	result, err := marshalJSON(job.Result)
	if err != nil {
		return err
	}

	query := `
UPDATE rigel_jobs
SET status = $2,
    payload = $3,
    result = $4,
    started_at = $5,
    finished_at = $6,
    retry_count = $7,
    error_message = $8,
    updated_at = NOW()
WHERE id = $1`

	if _, err := r.db.ExecContext(ctx, query,
		job.ID,
		job.Status,
		payload,
		result,
		job.StartedAt,
		job.FinishedAt,
		job.RetryCount,
		nullableString(job.ErrorMessage),
	); err != nil {
		return fmt.Errorf("update job: %w", err)
	}

	return nil
}

func (r *Repository) UpsertProduct(ctx context.Context, product model.Product) (model.Product, error) {
	attributes, err := marshalJSON(product.Attributes)
	if err != nil {
		return model.Product{}, err
	}
	rawPayload, err := marshalJSON(product.RawPayload)
	if err != nil {
		return model.Product{}, err
	}

	query := `
INSERT INTO rigel_products (
    source_platform, external_id, sku_id, title, subtitle, url, image_url,
    shop_name, shop_type, seller_name, region, price, currency, availability,
    attributes, raw_payload, first_seen_at, last_seen_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12, $13, $14,
    $15, $16, NOW(), NOW()
)
ON CONFLICT (source_platform, external_id)
DO UPDATE SET
    sku_id = EXCLUDED.sku_id,
    title = EXCLUDED.title,
    subtitle = EXCLUDED.subtitle,
    url = EXCLUDED.url,
    image_url = EXCLUDED.image_url,
    shop_name = EXCLUDED.shop_name,
    shop_type = EXCLUDED.shop_type,
    seller_name = EXCLUDED.seller_name,
    region = EXCLUDED.region,
    price = EXCLUDED.price,
    currency = EXCLUDED.currency,
    availability = EXCLUDED.availability,
    attributes = EXCLUDED.attributes,
    raw_payload = EXCLUDED.raw_payload,
    last_seen_at = NOW(),
    updated_at = NOW()
RETURNING id, first_seen_at, last_seen_at, created_at, updated_at`

	var id string
	if err := r.db.QueryRowContext(ctx, query,
		product.SourcePlatform,
		product.ExternalID,
		nullableString(product.SKUID),
		product.Title,
		nullableString(product.Subtitle),
		product.URL,
		nullableString(product.ImageURL),
		nullableString(product.ShopName),
		defaultShopType(product.ShopType),
		nullableString(product.SellerName),
		nullableString(product.Region),
		product.Price,
		defaultCurrency(product.Currency),
		defaultAvailability(product.Availability),
		attributes,
		rawPayload,
	).Scan(&id, &product.FirstSeenAt, &product.LastSeenAt, &product.CreatedAt, &product.UpdatedAt); err != nil {
		return model.Product{}, fmt.Errorf("upsert product: %w", err)
	}

	product.ID = model.ID(id)
	return product, nil
}

func (r *Repository) InsertPriceSnapshot(ctx context.Context, snapshot model.PriceSnapshot) (model.PriceSnapshot, error) {
	metadata, err := marshalJSON(snapshot.Metadata)
	if err != nil {
		return model.PriceSnapshot{}, err
	}

	query := `
INSERT INTO rigel_price_snapshots (product_id, source_platform, price, in_stock, captured_at, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id`

	capturedAt := snapshot.CapturedAt
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}

	var id string
	if err := r.db.QueryRowContext(ctx, query,
		snapshot.ProductID,
		snapshot.SourcePlatform,
		snapshot.Price,
		snapshot.InStock,
		capturedAt,
		metadata,
	).Scan(&id); err != nil {
		return model.PriceSnapshot{}, fmt.Errorf("insert price snapshot: %w", err)
	}

	snapshot.ID = model.ID(id)
	snapshot.CapturedAt = capturedAt
	return snapshot, nil
}

func (r *Repository) ListProducts(ctx context.Context, filter collector.ProductListFilter) ([]model.Product, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	query := `
SELECT id, source_platform::text, external_id, COALESCE(sku_id, ''), title, COALESCE(subtitle, ''), url,
       COALESCE(image_url, ''), COALESCE(shop_name, ''), shop_type::text, COALESCE(seller_name, ''), COALESCE(region, ''),
       price, currency, availability, attributes, raw_payload, first_seen_at, last_seen_at, created_at, updated_at
FROM rigel_products
WHERE source_platform = 'jd'
  AND ($1 = '' OR title ILIKE '%' || $1 || '%' OR external_id ILIKE '%' || $1 || '%')
  AND ($2 = '' OR COALESCE(attributes->>'category', '') = $2)
  AND ($3 = '' OR shop_type::text = $3)
  AND (NOT $4 OR shop_type::text = 'self_operated')
  AND (NOT $5 OR COALESCE(raw_payload->>'mock', 'false') <> 'true')
ORDER BY updated_at DESC
LIMIT $6`

	rows, err := r.db.QueryContext(ctx, query, filter.Keyword, filter.Category, string(filter.ShopType), filter.SelfOperatedOnly, filter.RealOnly, filter.Limit)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var product model.Product
		var sourcePlatform string
		var shopType string
		var attributes []byte
		var rawPayload []byte
		if err := rows.Scan(
			&product.ID,
			&sourcePlatform,
			&product.ExternalID,
			&product.SKUID,
			&product.Title,
			&product.Subtitle,
			&product.URL,
			&product.ImageURL,
			&product.ShopName,
			&shopType,
			&product.SellerName,
			&product.Region,
			&product.Price,
			&product.Currency,
			&product.Availability,
			&attributes,
			&rawPayload,
			&product.FirstSeenAt,
			&product.LastSeenAt,
			&product.CreatedAt,
			&product.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		product.SourcePlatform = model.SourcePlatform(sourcePlatform)
		product.ShopType = model.ShopType(shopType)
		if err := unmarshalJSON(attributes, &product.Attributes); err != nil {
			return nil, err
		}
		if err := unmarshalJSON(rawPayload, &product.RawPayload); err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products: %w", err)
	}

	return products, nil
}

func marshalJSON(value any) ([]byte, error) {
	if value == nil {
		return []byte(`{}`), nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return encoded, nil
}

func unmarshalJSON(data []byte, target *map[string]any) error {
	if len(data) == 0 {
		*target = map[string]any{}
		return nil
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	if *target == nil {
		*target = map[string]any{}
	}
	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func defaultShopType(shopType model.ShopType) string {
	if shopType == "" {
		return string(model.ShopTypeUnknown)
	}
	return string(shopType)
}

func defaultCurrency(currency string) string {
	if currency == "" {
		return "CNY"
	}
	return currency
}

func defaultAvailability(availability string) string {
	if availability == "" {
		return "unknown"
	}
	return availability
}
