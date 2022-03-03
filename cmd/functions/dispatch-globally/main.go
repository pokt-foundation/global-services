package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Pocket/global-dispatcher/common"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
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

	apps, err := getAllStakedApps(context.TODO(), db)

	if err != nil {
		return err
	}

	fmt.Printf("Apps: %d\n", len(apps))

	return nil
}

func getAllStakedApps(ctx context.Context, store common.ApplicationStore) ([]*common.Application, error) {
	apps, err := store.GetAllStakedApplications(ctx)
	if err != nil {
		return nil, err
	}

	pocketClient, err := pocket.NewPocket([]string{"<dispatch_url>"}, 2)
	if err != nil {
		return nil, err
	}

	ntApps, err := pocketClient.GetNetworkApplications(pocket.GetNetworkApplicationsInput{
		AppsPerPage: 3000,
		Page:        1,
	})
	if err != nil {
		return nil, err
	}

	for _, app := range ntApps {
		fmt.Printf("%v+", app)
	}

	return apps, nil
}

func main() {
	lambda.Start(LambdaHandler)
}
