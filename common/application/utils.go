package application

import (
	"context"
	"fmt"

	"github.com/Pocket/global-dispatcher/lib/pocket"
)

func GetStakedApplicationsOnDB(ctx context.Context, gigastaked bool, store ApplicationStore, pocketClient *pocket.PocketJsonRpcClient) ([]pocket.NetworkApplication, error) {
	var databaseApps []*Application
	var err error

	if gigastaked == true {
		databaseApps, err = store.GetGigastakedApplications(ctx)
		if err != nil {
			return nil, err
		}

		// Settlers provides traffic to new chains, need to be dispatched along with
		// gigastakes
		settlers, err := store.GetSettlersApplications(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range settlers {
			fmt.Printf("%+v\n", s)
		}

		databaseApps = append(databaseApps, settlers...)
	} else {
		databaseApps, err = store.GetStakedApplications(ctx)
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
