package gateway

import (
	"context"

	"github.com/Pocket/global-services/shared/gateway/models"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/pokt-foundation/pocket-go/provider"
)

// GetStakedApplicationsOnDB queries all apps on the database and only returns those that are staked on the network
func GetStakedApplicationsOnDB(ctx context.Context, gigastaked bool, store models.ApplicationStore, pocket *provider.Provider) ([]provider.GetAppOutput, []models.Application, error) {
	databaseApps, err := store.GetStakedApplications(ctx)
	if err != nil {
		return nil, nil, err
	}

	return FilterStakedAppsNotOnDB(databaseApps, pocket)
}

// GetGigastakedApplicationsOnDB returns applications that belong to a load balancer marked as 'gigastake'
func GetGigastakedApplicationsOnDB(ctx context.Context, store models.ApplicationStore, pocket *provider.Provider) ([]provider.GetAppOutput, []models.Application, error) {
	databaseApps, err := store.GetGigastakedApplications(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Settlers provides traffic to new chains, need to be dispatched along with
	// gigastakes
	settlers, err := store.GetSettlersApplications(ctx)
	if err != nil {
		return nil, nil, err
	}

	databaseApps = append(databaseApps, settlers...)

	return FilterStakedAppsNotOnDB(databaseApps, pocket)
}

// FilterStakedAppsNotOnDB takes a list of database apps and only returns those that are staked on the network
func FilterStakedAppsNotOnDB(dbApps []*models.Application, pocket *provider.Provider) ([]provider.GetAppOutput, []models.Application, error) {
	var stakedApps []provider.GetAppOutput
	var stakedAppsDB []models.Application

	networkApps, err := pocket.GetApps(&provider.GetAppsOptions{
		PerPage: 3000,
		Page:    1,
	})
	if err != nil {
		return nil, nil, err
	}

	publicKeyToApps := utils.SliceToMappedStruct(dbApps, func(app *models.Application) string {
		return app.GatewayAAT.ApplicationPublicKey
	})

	for _, ntApp := range networkApps.Result {
		if _, ok := publicKeyToApps[ntApp.PublicKey]; ok {
			stakedApps = append(stakedApps, ntApp)
			stakedAppsDB = append(stakedAppsDB, *publicKeyToApps[ntApp.PublicKey])
		}
	}
	return stakedApps, stakedAppsDB, nil
}
