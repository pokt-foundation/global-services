package database

import (
	"context"

	"github.com/Pocket/global-services/shared/utils"
	dbclient "github.com/pokt-foundation/db-client/client"
	"github.com/pokt-foundation/pocket-go/provider"
	"github.com/pokt-foundation/portal-db/types"
	"golang.org/x/exp/slices"
)

// PostgresDBClient holds the phd client lib and additional methods for filtering applications
type PostgresDBClient struct {
	*dbclient.DBClient
}

// NewPHDClient returns an augmented HTTP client for the pocket http db which adheres which adds app filters
func NewPHDClient(config dbclient.Config) (*PostgresDBClient, error) {
	client, err := dbclient.NewDBClient(config)
	if err != nil {
		return nil, err
	}

	return &PostgresDBClient{
		client,
	}, nil
}

// GetStakedApplications returns all the staked applications on the database
func (pg PostgresDBClient) GetStakedApplications(ctx context.Context, pocket *provider.Provider) ([]*provider.App, []*types.Application, error) {
	apps, err := pg.GetApplications(ctx)
	if err != nil {
		return nil, nil, err
	}

	return FilterStakedAppsNotOnDB(apps, pocket)
}

// GetAppsFromList returns only the applications provided in the slice of app IDs
func (pg PostgresDBClient) GetAppsFromList(ctx context.Context, appIDs []string) ([]*types.Application, error) {
	apps, err := pg.GetApplications(ctx)
	if err != nil {
		return nil, err
	}

	return filter(apps, func(a *types.Application) bool {
		return slices.Contains(appIDs, a.ID)
	}), nil
}

// FilterStakedAppsNotOnDB takes a list of database apps and only returns those that are staked on the network
func FilterStakedAppsNotOnDB(dbApps []*types.Application, pocket *provider.Provider) ([]*provider.App, []*types.Application, error) {
	var stakedApps []*provider.App
	var stakedAppsDB []*types.Application

	networkApps, err := pocket.GetApps(&provider.GetAppsOptions{
		PerPage: 3000,
		Page:    1,
	})
	if err != nil {
		return nil, nil, err
	}

	publicKeyToApps := utils.SliceToMappedStruct(dbApps, func(app *types.Application) string {
		return app.GatewayAAT.ApplicationPublicKey
	})

	for _, ntApp := range networkApps.Result {
		if _, ok := publicKeyToApps[ntApp.PublicKey]; ok {
			stakedApps = append(stakedApps, ntApp)
			stakedAppsDB = append(stakedAppsDB, publicKeyToApps[ntApp.PublicKey])
		}
	}
	return stakedApps, stakedAppsDB, nil
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
