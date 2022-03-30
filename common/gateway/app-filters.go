package gateway

import (
	"context"

	"github.com/Pocket/global-dispatcher/lib/utils"
	"github.com/pokt-foundation/pocket-go/pkg/provider"
)

func GetStakedApplicationsOnDB(ctx context.Context, gigastaked bool, store ApplicationStore, pocket *provider.JSONRPCProvider) ([]provider.GetAppResponse, []Application, error) {
	var databaseApps []*Application
	var err error

	if gigastaked == true {
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

func FilterStakedAppsNotOnDB(dbApps []*Application, ntApps []provider.GetAppResponse) ([]provider.GetAppResponse, []Application) {
	var stakedApps []provider.GetAppResponse
	var stakedAppsDB []Application

	publicKeyToApps := utils.SliceToMappedStruct(dbApps, func(app *Application) string {
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
