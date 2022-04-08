package gateway

import (
	"context"

	"github.com/Pocket/global-dispatcher/common/gateway/models"
	"github.com/Pocket/global-dispatcher/lib/utils"
	"github.com/pokt-foundation/pocket-go/pkg/provider"
)

func GetStakedApplicationsOnDB(ctx context.Context, gigastaked bool, store models.ApplicationStore, pocket *provider.JSONRPCProvider) ([]provider.GetAppOutput, []models.Application, error) {
	var databaseApps []*models.Application
	var err error

	if gigastaked {
		databaseApps, err = store.GetGigastakedApplications(ctx)
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
	} else {
		databaseApps, err = store.GetStakedApplications(ctx)
		if err != nil {
			return nil, nil, err
		}
	}

	networkApps, err := pocket.GetApps(0, &provider.GetAppsOptions{
		PerPage: 3000,
		Page:    1,
	})
	if err != nil {
		return nil, nil, err
	}

	networkAppsInDB, stakedAppsDB := FilterStakedAppsNotOnDB(databaseApps, networkApps.Result)

	return networkAppsInDB, stakedAppsDB, nil
}

func FilterStakedAppsNotOnDB(dbApps []*models.Application, ntApps []provider.GetAppOutput) ([]provider.GetAppOutput, []models.Application) {
	var stakedApps []provider.GetAppOutput
	var stakedAppsDB []models.Application

	publicKeyToApps := utils.SliceToMappedStruct(dbApps, func(app *models.Application) string {
		return app.GatewayAAT.ApplicationPublicKey
	})

	for _, ntApp := range ntApps {

		if _, ok := publicKeyToApps[ntApp.PublicKey]; ok {
			stakedApps = append(stakedApps, ntApp)
			stakedAppsDB = append(stakedAppsDB, *publicKeyToApps[ntApp.PublicKey])
		}
	}
	return stakedApps, stakedAppsDB
}
