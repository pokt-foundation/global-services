package database

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	dbclient "github.com/pokt-foundation/db-client/client"
	"github.com/pokt-foundation/pocket-go/provider"
	"github.com/pokt-foundation/utils-go/mock-client"
	"github.com/stretchr/testify/require"
)

const (
	testPHDURL          = "https://test-db.com"
	testPHDAPIKey       = "test-api-key-123"
	testNetworkURL      = "https://test-network.com"
	dbAppsEndpoint      = "application"
	networkAppsEndpoint = "/v1/query/apps"
)

func newPhdClient() *PostgresDBClient {
	client, err := NewPHDClient(dbclient.Config{
		BaseURL: testPHDURL,
		Timeout: 2 * time.Second,
		Retries: 0,
		Version: dbclient.V1,
		APIKey:  testPHDAPIKey,
	})
	if err != nil {
		panic(err)
	}

	return client
}

func TestCache_GetStakedApplications(t *testing.T) {
	c := require.New(t)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s/v1/%s", testPHDURL, dbAppsEndpoint),
		http.StatusOK, "../../testdata/db_apps.json")

	mock.AddMockedResponseFromFile(http.MethodPost, fmt.Sprintf("%s%s", testNetworkURL, networkAppsEndpoint),
		http.StatusOK, "../../testdata/network_apps.json")

	rpcProvider := provider.NewProvider(testNetworkURL, []string{testNetworkURL})

	phdClient := newPhdClient()

	_, dbApps, err := phdClient.GetStakedApplications(context.Background(), rpcProvider)

	c.NoError(err)

	c.Equal(len(dbApps), 3)
	c.Equal(dbApps[0].GatewayAAT.ApplicationPublicKey, "test_app_pub_key_c88022df98fd0423823df6d24af19031a1b319d44e3ed3a")
}

func TestCache_GetAppsFromList(t *testing.T) {
	c := require.New(t)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s/v1/%s", testPHDURL, dbAppsEndpoint),
		http.StatusOK, "../../testdata/db_apps.json")

	phdClient := newPhdClient()

	appID := "test_id_4e6d55dadac3e4a4"
	apps, err := phdClient.GetAppsFromList(context.Background(), []string{appID})

	c.NoError(err)

	c.Equal(len(apps), 1)
	c.Equal(apps[0].ID, appID)
}
