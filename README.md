# rigel-jd-collector

JD collector service for daily raw product sampling and price snapshots.

## Language

Go

## Current Stage

Phase 3 MVP, now aligned more explicitly with the `daily price catalog` goal.

## Intended Role

- search JD by canonical-model-oriented keywords
- store raw product records
- append daily price snapshots
- provide enough data for canonical model aggregation
- prefer only the first few useful self-operated items instead of deep paging

## Implemented

- replaceable JD search adapter interface
- deterministic mock JD client for local development
- PostgreSQL persistence for `products`, `price_snapshots`, and `jobs`
- basic dedupe through `products(source_platform, external_id)` upsert
- smart batch collection with skip and risk-aware abort

## Routes

- `GET /healthz`
- `POST /api/v1/collect/search`
- `POST /api/v1/collect/batch`
- `GET /api/v1/products`
- `GET /api/v1/jobs`
- `GET /api/v1/jobs/{id}`
- `POST /api/v1/jobs/{id}/retry`

## Notes

- The goal is not deep catalog completeness. The goal is reliable daily price sampling.
- Self-operated JD records are currently preferred when available.

## TODO / MOCK

- TODO: support a clearer daily sampling workflow per canonical keyword set
