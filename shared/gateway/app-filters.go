package gateway

import (
	"context"

	"github.com/Pocket/global-services/shared/database"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/pokt-foundation/pocket-go/provider"
	"github.com/pokt-foundation/portal-db/types"
)

// GetApplicationsFromDB returns all the needed applications to perform checks on
func GetApplicationsFromDB(ctx context.Context, phdClient *database.PostgresDBClient, pocket *provider.Provider, apps []string) ([]*provider.App, []*types.Application, error) {
	databaseApps, err := phdClient.GetGigastakedApplications(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Some apps do not belong to a gigastake but there's still the need to perform checks on them
	singleApps := []*types.Application{}
	if len(apps) > 0 {
		singleApps, err = phdClient.GetAppsFromList(ctx, apps)
		if err != nil {
			return nil, nil, err
		}
	}
	databaseApps = append(databaseApps, singleApps...)

	return FilterStakedAppsNotOnDB(databaseApps, pocket)
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
