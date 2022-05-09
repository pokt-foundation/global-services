package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	db "github.com/Pocket/global-services/cherry-picker/database"
	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/cache"
	"github.com/Pocket/global-services/shared/database"
	"github.com/Pocket/global-services/shared/environment"
	shared "github.com/Pocket/global-services/shared/error"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pokt-foundation/pocket-go/provider"
)

var (
	rpcURL                  = environment.GetString("RPC_URL", "")
	dispatchURLs            = strings.Split(environment.GetString("DISPATCH_URLS", ""), ",")
	cherryPickerConnections = strings.Split(environment.GetString("CHERRY_PICKER_CONNECTIONS	", ""), ",")
	defaultTimeOut          = environment.GetInt64("DEFAULT_TIMEOUT", 8)
	redisConnectionStrings  = environment.GetString("REDIS_REGION_CONNECTION_STRINGS", "")
	isRedisCluster          = environment.GetBool("IS_REDIS_CLUSTER", true)
	mongoConnectionString   = environment.GetString("MONGODB_CONNECTION_STRING", "")
	mongoDatabase           = environment.GetString("MONGODB_DATABASE", "gateway")
)

type snapCherryPicker struct {
	caches       map[string]*cache.Redis
	appDB        *database.Mongo
	cherryPickDB []*db.CherryPickerPostgres
	rpcProvider  *provider.Provider
}

func (sn *snapCherryPicker) init(ctx context.Context) error {
	if err := sn.initRegionCaches(ctx); err != nil {
		return err
	}

	mongodb, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return errors.New("error connecting to mongo: " + err.Error())
	}
	sn.appDB = mongodb

	rpcProvider := provider.NewProvider(rpcURL, dispatchURLs)
	rpcProvider.UpdateRequestConfig(0, time.Duration(defaultTimeOut)*time.Second)
	sn.rpcProvider = rpcProvider

	for _, connString := range cherryPickerConnections {
		connection, err := database.NewPostgresDatabase(ctx, &database.PostgresOptions{
			Connection:  connString,
			MinPoolSize: 10,
			MaxPoolSize: 10,
		})
		if err != nil {
			return err
		}
		sn.cherryPickDB = append(sn.cherryPickDB, &db.CherryPickerPostgres{Db: connection})
	}

	return nil
}

func (sn *snapCherryPicker) initRegionCaches(ctx context.Context) error {
	var cacheRegionConns map[string]string

	if err := json.Unmarshal([]byte(redisConnectionStrings), &cacheRegionConns); err != nil {
		return err
	}

	if len(cacheRegionConns) == 0 {
		return shared.ErrNoCacheClientProvided
	}

	conns := make([]string, len(cacheRegionConns))
	for _, connStr := range cacheRegionConns {
		conns = append(conns, connStr)
	}

	caches, err := cache.ConnectoCacheClients(ctx, conns, "", isRedisCluster)
	if err != nil {
		return err
	}

	regionCaches := make(map[string]*cache.Redis)
	for region, connStr := range cacheRegionConns {
		idx := slices.IndexFunc(caches, func(ch *cache.Redis) bool {
			return ch.Addrs()[0] == connStr
		})
		regionCaches[region] = caches[idx]
	}

	if err != nil {
		return err
	}
	for key, ch := range regionCaches {
		fmt.Println("key", key, "addr", ch.Addrs()[0])
	}
	sn.caches = regionCaches

	return nil
}

func (sn *snapCherryPicker) snapCherryPickerData(ctx context.Context) error {

	return nil
}

func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	var statusCode int
	snapCherryPickerData := &snapCherryPicker{}

	err := snapCherryPickerData.init(ctx)
	if err != nil {
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	err = snapCherryPickerData.snapCherryPickerData(ctx)
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
