package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/exp/slices"

	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/cache"
	"github.com/Pocket/global-services/shared/environment"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	redisConnectionStrings = environment.GetString("REDIS_REGION_CONNECTION_STRINGS", "")
	isRedisCluster         = environment.GetBool("IS_REDIS_CLUSTER", true)
)

type snapCherryPickerData struct {
	caches map[string]*cache.Redis
}

func (sn *snapCherryPickerData) init(ctx context.Context) error {
	regionCaches, err := initRegionCaches(ctx)
	if err != nil {
		return err
	}
	for key, ch := range regionCaches {
		fmt.Println("key", key, "addr", ch.Addrs()[0])
	}

	sn.caches = regionCaches

	return nil
}

func initRegionCaches(ctx context.Context) (map[string]*cache.Redis, error) {
	var cacheRegionConns map[string]string

	if err := json.Unmarshal([]byte(redisConnectionStrings), &cacheRegionConns); err != nil {
		return nil, err
	}

	conns := make([]string, len(cacheRegionConns))
	for _, connStr := range cacheRegionConns {
		conns = append(conns, connStr)
	}

	caches, err := cache.ConnectoCacheClients(ctx, conns, "", isRedisCluster)
	if err != nil {
		return nil, err
	}

	regionCaches := make(map[string]*cache.Redis)
	for region, connStr := range cacheRegionConns {
		idx := slices.IndexFunc(caches, func(ch *cache.Redis) bool {
			return ch.Addrs()[0] == connStr
		})
		regionCaches[region] = caches[idx]
	}

	return regionCaches, nil
}

func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	var statusCode int
	snapCherryPickerData := &snapCherryPickerData{}

	err := snapCherryPickerData.init(ctx)
	if err != nil {
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(statusCode, map[string]interface{}{
		"ok": true,
	}), err
}

func main() {
	lambda.Start(LambdaHandler)
}
