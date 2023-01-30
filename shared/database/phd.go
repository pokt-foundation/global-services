package database

import (
	"context"

	dbclient "github.com/pokt-foundation/db-client/client"
	"github.com/pokt-foundation/portal-db/types"
	"golang.org/x/exp/slices"
)

type PostgresDBClient struct {
	*dbclient.DBClient
}

// NewPostgresDBClient returns an augmented HTTP client for the pocket http db which adheres with the
// current model filters interface.
func NewPostgresDBClient(config dbclient.Config) (*PostgresDBClient, error) {
	client, err := dbclient.NewDBClient(config)
	if err != nil {
		return nil, err
	}

	return &PostgresDBClient{
		client,
	}, nil
}

// GetStakedApplications returns all the staked applications on the database
func (pg PostgresDBClient) GetStakedApplications(ctx context.Context) ([]*types.Application, error) {
	apps, err := pg.GetApplications(ctx)
	if err != nil {
		return nil, err
	}

	return filter(apps, func(a *types.Application) bool {
		return !a.Dummy
	}), nil
}

func (pg PostgresDBClient) GetGigastakedApplications(ctx context.Context) ([]*types.Application, error) {
	lbs, err := pg.GetLoadBalancers(ctx)
	if err != nil {
		return nil, err
	}

	gigastakeLBs := filter(lbs, func(lb *types.LoadBalancer) bool {
		return lb.Gigastake
	})

	var gigastakeApps []*types.Application
	for _, lb := range gigastakeLBs {
		gigastakeApps = append(gigastakeApps, lb.Applications...)
	}

	return gigastakeApps, nil
}

func (pg PostgresDBClient) GetAppsFromList(ctx context.Context, appIDs []string) ([]*types.Application, error) {
	apps, err := pg.GetApplications(ctx)
	if err != nil {
		return nil, err
	}

	return filter(apps, func(a *types.Application) bool {
		return slices.Contains(appIDs, a.ID)
	}), nil
}

// filter calls a function on each element of a slice, returning a new slice whose elements returned true from the function
func filter[T any](slice []T, f func(T) bool) []T {
	var n []T
	for _, e := range slice {
		if f(e) {
			n = append(n, e)
		}
	}
	return n
}
