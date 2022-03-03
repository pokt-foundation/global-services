package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Pocket/global-dispatcher/common"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-redis/redis/v8"
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration
type Response events.APIGatewayProxyResponse

// Handler is our lambda handler invoked by the `lambda.Start` function call
func LambdaHandler(ctx context.Context) (Response, error) {
	var buf bytes.Buffer

	err := Handler()

	if err != nil {
		fmt.Println(err)
	}

	body, err := json.Marshal(map[string]interface{}{
		"message": "Go Serverless v1.0! Your function executed successfully!",
	})
	if err != nil {
		return Response{StatusCode: 404}, err
	}
	json.HTMLEscape(&buf, body)

	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            buf.String(),
		Headers: map[string]string{
			"Content-Type":           "application/json",
			"X-MyCompany-Func-Reply": "hello-handler",
		},
	}

	return resp, nil
}

func Handler() error {
	// 1 - Connect to mongodb
	// 2 - Get the production commit hash
	// 3 - Get all staked applications
	// 4 - Validate them
	// 5 - for each app and blockchain, call the dispatch on a goroutine
	// 5.1 - set the redis value for the dispatch given

	db, err := database.ClientFromURI(context.TODO(), "mongodb://mongouser:mongopassword@db:27017/gateway?authSource=admin", "gateway")
	if err != nil {
		return err
	}

	pocketClient, err := pocket.NewPocketClient("", []string{"", ""}, 2)
	if err != nil {
		return err
	}

	apps, err := getAllStakedApplicationsOnDB(context.TODO(), db, *pocketClient)
	if err != nil {
		return err
	}

	session, err := pocketClient.DispatchSession(pocket.DispatchInput{
		AppPublicKey: apps[0].PublicKey,
		Chain:        apps[0].Chains[0],
	})
	if err != nil {
		return err
	}

	redisClient, err := cache.NewRedisClient(cache.RedisClientOptions{
		BaseOptions: &redis.Options{
			Addr:     "cache:6379",
			Password: "",
			DB:       0,
		},
		KeyPrefix: "klk-",
	})
	if err != nil {
		return err
	}

	if err := redisClient.SetJSON(context.TODO(), "session-test", session, 3600); err != nil {
		return err
	}

	if err != nil {
		return err
	}

	return nil
}

func getAllStakedApplicationsOnDB(ctx context.Context, store common.ApplicationStore, pocketClient pocket.Pocket) ([]common.NetworkApplication, error) {
	databaseApps, err := store.GetAllStakedApplications(ctx)
	if err != nil {
		return nil, err
	}

	networkApps, err := pocketClient.GetNetworkApplications(pocket.GetNetworkApplicationsInput{
		AppsPerPage: 3000,
		Page:        1,
	})
	if err != nil {
		return nil, err
	}

	return filterStakedAppsNotOnDB(databaseApps, networkApps), nil
}

func filterStakedAppsNotOnDB(dbApps []*common.Application, ntApps []common.NetworkApplication) []common.NetworkApplication {
	var stakedAppsOnDB []common.NetworkApplication
	publicKeyToApps := mapApplicationsToPublicKey(dbApps)

	for _, ntApp := range ntApps {
		if _, ok := publicKeyToApps[ntApp.PublicKey]; ok {
			stakedAppsOnDB = append(stakedAppsOnDB, ntApp)
		}
	}

	return stakedAppsOnDB
}

func mapApplicationsToPublicKey(applications []*common.Application) map[string]*common.Application {
	applicationsMap := make(map[string]*common.Application)

	for _, application := range applications {
		applicationsMap[application.GatewayAAT.ApplicationPublicKey] = application
	}

	return applicationsMap
}

func main() {
	lambda.Start(LambdaHandler)
}
