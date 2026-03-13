package jdclient

import (
	"context"

	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
)

// Client isolates JD search integration details behind a replaceable adapter.
type Client interface {
	SearchProducts(ctx context.Context, query model.SearchQuery) ([]model.Product, error)
}
