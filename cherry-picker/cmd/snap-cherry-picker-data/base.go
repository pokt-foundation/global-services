package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/cache"
	"github.com/Pocket/global-services/shared/environment"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	redisConnectionStrings = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", ""), ",")
	isRedisCluster         = environment.GetBool("IS_REDIS_CLUSTER", true)
)

func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	var statusCode int

	err := cherryPickerService(ctx)
	if err != nil {
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(statusCode, map[string]interface{}{
		"ok": true,
	}), err
}

func cherryPickerService(ctx context.Context) error {
	_, err := cache.ConnectoCacheClients(ctx, redisConnectionStrings, "", isRedisCluster)

	return err
}

func main() {
	lambda.Start(LambdaHandler)
}
