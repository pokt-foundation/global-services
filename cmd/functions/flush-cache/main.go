package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Pocket/global-dispatcher/common/apigateway"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	redisConnectionStrings = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", ""), ",")
	isRedisCluster         = environment.GetBool("IS_REDIS_CLUSTER", true)

	headers = map[string]string{
		"Content-Type": "application/json",
	}
)

// LambdaHandler manages the DispatchSession call to return as an APIGatewayProxyResponse
func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	var statusCode int

	err := FlushAll(ctx)
	if err != nil {
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(statusCode, map[string]interface{}{
		"ok": true,
	}), err
}

func FlushAll(ctx context.Context) error {
	cacheClients, err := cache.ConnectoCacheClients(redisConnectionStrings, "", isRedisCluster)

	if err != nil {
		return errors.New("error connecting to redis: " + err.Error())
	}

	return cache.RunFunctionOnAllClients(cacheClients, func(ins *cache.Redis) error {
		return ins.Client.FlushAll(ctx).Err()
	})
}

func main() {
	lambda.Start(LambdaHandler)
}
