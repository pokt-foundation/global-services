package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Pocket/global-dispatcher/common/application"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/common/gateway"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	rpcURL                 = environment.GetString("RPC_URL", "")
	dispatchURLs           = strings.Split(environment.GetString("DISPATCH_URLS", ""), ",")
	redisConnectionStrings = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", ""), ",")
	mongoConnectionString  = environment.GetString("MONGODB_CONNECTION_STRING", "")
	mongoDatabase          = environment.GetString("MONGODB_DATABASE", "")
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
	fmt.Println("Starting")
	ctx := context.Background()

	db, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return err
	}
	pocketClient, err := pocket.NewPocketClient(rpcURL, dispatchURLs, 2)
	if err != nil {
		return err
	}

	apps, err := application.GetAllStakedApplicationsOnDB(ctx, db, *pocketClient)
	if err != nil {
		return err
	}

	commitHash, err := gateway.GetGatewayCommitHash()
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

	redisClients, err := cache.GetCacheClients(redisConnectionStrings, commitHash)
	if err != nil {
		return err
	}

	for _, client := range redisClients {
		if err := client.SetJSON(context.TODO(), "-session-test", session, 3600); err != nil {
			fmt.Println(err)
		}
	}

	if err != nil {
		return err
	}

	return nil
}

func main() {
	lambda.Start(LambdaHandler)
}
