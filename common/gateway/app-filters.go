package gateway

import (
	"context"

	"github.com/Pocket/global-dispatcher/common/gateway/models"
	"github.com/Pocket/global-dispatcher/lib/utils"
	"github.com/pokt-foundation/pocket-go/provider"
)

func GetStakedApplicationsOnDB(ctx context.Context, gigastaked bool, store models.ApplicationStore, pocket *provider.Provider) ([]provider.GetAppOutput, []models.Application, error) {
	databaseApps, err := store.GetStakedApplications(ctx)
	if err != nil {
		return nil, nil, err
	}

	return FilterStakedAppsNotOnDB(databaseApps, pocket)
}

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

func FilterStakedAppsNotOnDB(dbApps []*models.Application, pocket *provider.Provider) ([]provider.GetAppOutput, []models.Application, error) {
	var stakedApps []provider.GetAppOutput
	var stakedAppsDB []models.Application

	networkApps, err := pocket.GetApps(0, &provider.GetAppsOptions{
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
