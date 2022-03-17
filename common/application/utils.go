package application

import (
	"context"

	"github.com/Pocket/global-dispatcher/lib/pocket"
)

func GetAllStakedApplicationsOnDB(ctx context.Context, gigastake bool, store ApplicationStore, pocketClient pocket.PocketJsonRpcClient) ([]pocket.NetworkApplication, error) {
	var databaseApps []*Application
	var err error

	if gigastake == true {
		databaseApps, err = store.GetAllGigastakeApplications(ctx)
		if err != nil {
			return nil, err
		}
	} else {

		databaseApps, err = store.GetAllStakedApplications(ctx)
		if err != nil {
			return nil, err
		}
	}

	networkApps, err := pocketClient.GetNetworkApplications(pocket.GetNetworkApplicationsInput{
		AppsPerPage: 3000,
		Page:        1,
	})
	if err != nil {
		return nil, err
	}

	return FilterStakedAppsNotOnDB(databaseApps, networkApps), nil
}

func FilterStakedAppsNotOnDB(dbApps []*Application, ntApps []pocket.NetworkApplication) []pocket.NetworkApplication {
	var stakedAppsOnDB []pocket.NetworkApplication
	publicKeyToApps := mapApplicationsToPublicKey(dbApps)

	for _, ntApp := range ntApps {
		if _, ok := publicKeyToApps[ntApp.PublicKey]; ok {
			stakedAppsOnDB = append(stakedAppsOnDB, ntApp)
		}
	}

	return stakedAppsOnDB
}

func mapApplicationsToPublicKey(applications []*Application) map[string]*Application {
	applicationsMap := make(map[string]*Application)

	for _, application := range applications {
		applicationsMap[application.GatewayAAT.ApplicationPublicKey] = application
	}

	return applicationsMap
}
