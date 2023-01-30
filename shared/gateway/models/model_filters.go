package models

import (
	"context"

	"github.com/pokt-foundation/portal-db/types"
)

type AppFilters interface {
	GetApplications(ctx context.Context) ([]*types.Application, error)
	GetStakedApplications(ctx context.Context) ([]*types.Application, error)
	GetGigastakedApplications(ctx context.Context) ([]*types.Application, error)
	GetSettlersApplications(ctx context.Context) ([]*types.Application, error)
	GetAppsFromList(ctx context.Context, appIDs []string) ([]*types.Application, error)
}

// Made for compatibility with the current mongodb interfaces
// TODO: Remove interfaces below once mongodb is deprecated
type LoadBalancerFilters interface {
	GetLoadBalancers(ctx context.Context) ([]*types.Application, error)
}

type BlockchainFilters interface {
	GetBlockchains(ctx context.Context) ([]*types.Application, error)
}
