package gateway

import (
	"context"

	"github.com/Pocket/global-services/shared/gateway/models"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/pokt-foundation/pocket-go/provider"
)

// GetStakedApplicationsOnDB queries all apps on the database and only returns those that are staked on the network
func GetStakedApplicationsOnDB(ctx context.Context, gigastaked bool, store models.ApplicationStore, pocket *provider.Provider) ([]provider.App, []models.Application, error) {
	databaseApps, err := store.GetStakedApplications(ctx)
	if err != nil {
		return nil, nil, err
	}

	return FilterStakedAppsNotOnDB(databaseApps, pocket)
}

// GetApplicationsFromDB returns all the needed applications to perform checks on
func GetApplicationsFromDB(ctx context.Context, store models.ApplicationStore, pocket *provider.Provider, apps []string) ([]provider.App, []models.Application, error) {
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

	// Some apps do not belong to either a gigastake or settle but still need to perform chekcs on
	singleApps := []*models.Application{}
	if len(apps) > 0 {
		singleApps, err = store.GetAppsFromList(ctx, apps)
		if err != nil {
			return nil, nil, err
		}
	}

	databaseApps = append(databaseApps, settlers...)
	databaseApps = append(databaseApps, singleApps...)

	return FilterStakedAppsNotOnDB(databaseApps, pocket)
}

// FilterStakedAppsNotOnDB takes a list of database apps and only returns those that are staked on the network
func FilterStakedAppsNotOnDB(dbApps []*models.Application, pocket *provider.Provider) ([]provider.App, []models.Application, error) {
	var stakedApps []provider.App
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
			stakedApps = append(stakedApps, *ntApp)
			stakedAppsDB = append(stakedAppsDB, *publicKeyToApps[ntApp.PublicKey])
		}
	}
	return stakedApps, stakedAppsDB, nil
}
