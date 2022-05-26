package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/cache"
	"github.com/Pocket/global-services/shared/environment"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	redisConnectionStrings = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", ""), ",")
	isRedisCluster         = environment.GetBool("IS_REDIS_CLUSTER", true)
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

// FlushAll deletes all the keys of the redis instance
func FlushAll(ctx context.Context) error {
	cacheClients, err := cache.ConnectoCacheClients(ctx, redisConnectionStrings, "", isRedisCluster)
	if err != nil {
		return errors.New("error connecting to redis: " + err.Error())
	}

	return utils.RunFnOnSliceSingleFailure(cacheClients, func(ins *cache.Redis) error {
		return ins.Client.FlushAll(ctx).Err()
	})
}

func main() {
	lambda.Start(LambdaHandler)
}
