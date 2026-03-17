# rigel-jd-collector

JD collector service for querying JD Union data and writing raw product/price records into PostgreSQL.

## Language

Go

## Current Stage

Phase 3 MVP, now aligned more explicitly with the `JD Union query -> raw sample storage` goal.

## Intended Role

- call JD Union/OpenAPI search interfaces with canonical-model-oriented keywords
- store raw product records
- append daily price snapshots
- provide enough data for canonical model aggregation in `rigel-build-engine`
- keep collector responsibility limited to query and persistence

## Implemented

- replaceable JD Union client adapter interface
- local mock adapter for development before real JD Union credentials are available
- PostgreSQL persistence for `products`, `price_snapshots`, and `jobs`
- basic dedupe through `products(source_platform, external_id)` upsert
- batch collection for a fixed keyword set

## Routes

- `GET /healthz`
- `POST /api/v1/collect/search`
- `POST /api/v1/collect/batch`
- `GET /api/v1/products`
- `GET /api/v1/jobs`
- `GET /api/v1/jobs/{id}`
- `POST /api/v1/jobs/{id}/retry`

## Notes

- Current scope assumes JD data should come from JD Union/OpenAPI rather than browser scraping.
- The goal is not deep catalog completeness. The goal is reliable daily price sampling and storage.

## TODO / MOCK

- TODO: wire a verified JD Union client once official credentials are available
- MOCK: local mock adapter remains in place until JD Union access is configured
