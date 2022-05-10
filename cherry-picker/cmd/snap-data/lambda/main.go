package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	cpicker "github.com/Pocket/global-services/cherry-picker"
	db "github.com/Pocket/global-services/cherry-picker/database"
	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/cache"
	"github.com/Pocket/global-services/shared/database"
	"github.com/Pocket/global-services/shared/environment"
	shared "github.com/Pocket/global-services/shared/error"
	"github.com/Pocket/global-services/shared/gateway"
	"github.com/aws/aws-lambda-go/events"
	"github.com/pokt-foundation/pocket-go/provider"
)

var (
	rpcURL                  = environment.GetString("RPC_URL", "")
	dispatchURLs            = strings.Split(environment.GetString("DISPATCH_URLS", ""), ",")
	cherryPickerConnections = strings.Split(environment.GetString("CHERRY_PICKER_CONNECTIONS	", "postgres://pguser:pgpassword@localhost:5432/gateway"), ",")
	defaultTimeOut          = environment.GetInt64("DEFAULT_TIMEOUT", 8)
	redisConnectionStrings  = environment.GetString("REDIS_REGION_CONNECTION_STRINGS", "{\"localhost\": \"localhost:6379\"}")
	isRedisCluster          = environment.GetBool("IS_REDIS_CLUSTER", false)
	mongoConnectionString   = environment.GetString("MONGODB_CONNECTION_STRING", "")
	mongoDatabase           = environment.GetString("MONGODB_DATABASE", "gateway")
	concurrency             = environment.GetInt64("CONCURRENCY", 1)
	successKey              = environment.GetString("SUCCESS_KEY", "success-hits")
	failuresKey             = environment.GetString("SUCCESS_KEY", "failure-hits")
	failureKey              = environment.GetString("SUCCESS_KEY", "failure")
)

type applicationData struct {
	ServiceLog cpicker.ServiceLog
	Successes  int
	Failures   int
	Failure    bool
}

type regionData struct {
	Cache   *cache.Redis
	AppData map[string]*applicationData
}

type snapCherryPicker struct {
	regionsData  map[string]*regionData
	caches       []*cache.Redis
	appDB        *database.Mongo
	cherryPickDB []*db.CherryPickerPostgres
	rpcProvider  *provider.Provider
	apps         []provider.GetAppOutput
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

	apps, _, err := gateway.GetGigastakedApplicationsOnDB(ctx, mongodb, rpcProvider)
	if err != nil {
		return errors.New("error obtaining staked apps on db: " + err.Error())
	}
	sn.apps = apps

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

	for region, connStr := range cacheRegionConns {
		idx := slices.IndexFunc(caches, func(ch *cache.Redis) bool {
			return ch.Addrs()[0] == connStr
		})

		ch := caches[idx]
		ch.Name = region
		sn.regionsData[region] = &regionData{
			Cache:   ch,
			AppData: make(map[string]*applicationData),
		}
		sn.caches = append(sn.caches, ch)
	}

	return nil
}

func (sn *snapCherryPicker) snapCherryPickerData(ctx context.Context) error {
	sn.getAppsRegionsData(ctx)

	return nil
}

func (sn *snapCherryPicker) getAppsRegionsData(ctx context.Context) {
	cache.RunFunctionOnAllClients(sn.caches, func(cl *cache.Redis) error {
		sn.getServiceLogData(ctx, cl)
		sn.getSuccessAndFailureData(ctx, cl)
		return nil
	})
}

func (sn *snapCherryPicker) getServiceLogData(ctx context.Context, cl *cache.Redis) error {
	serviceLogKeys, err := cl.Client.Keys(ctx, "*service*").Result()
	if err != nil {
		return err
	}

	results, err := cl.Client.MGet(ctx, serviceLogKeys...).Result()
	if err != nil {
		return err
	}
	for idx, rawServiceLog := range results {
		appDataKey := geAppKeyFromLog(serviceLogKeys[idx])
		sn.regionsData[cl.Name].AppData[appDataKey] = &applicationData{}

		if err := cache.UnMarshallJSONResult(rawServiceLog, nil,
			&sn.regionsData[cl.Name].AppData[appDataKey].ServiceLog); err != nil {
			// TODO: Log
		}
	}

	return nil
}

func (sn *snapCherryPicker) getSuccessAndFailureData(ctx context.Context, cl *cache.Redis) error {
	successKeys, err := cl.Client.Keys(ctx, "*"+successKey).Result()
	if err != nil {
		return err
	}
	failuresKeys, err := cl.Client.Keys(ctx, "*"+failuresKey).Result()
	if err != nil {
		return err
	}
	failureKeys, err := cl.Client.Keys(ctx, "*"+failureKey).Result()
	if err != nil {
		return err
	}

	allKeys := append(successKeys, failuresKeys...)
	allKeys = append(allKeys, failureKeys...)
	results, err := cl.Client.MGet(ctx, allKeys...).Result()
	if err != nil {
		return err
	}

	for idx, rawResult := range results {
		key := allKeys[idx]
		appDataKey := geAppKeyFromLog(key)
		region := sn.regionsData[cl.Name].AppData[appDataKey]
		result, _ := cache.GetStringResult(rawResult, nil)
		if region == nil {
			continue
		}

		switch {
		case strings.Contains(key, successKey):
			sessionKey := strings.Split(key, "-")[2]
			successes, err := strconv.Atoi(result)
			if err != nil || region.ServiceLog.SessionKey != sessionKey {
				continue
			}
			region.Successes = successes
		case strings.Contains(key, failuresKey):
			sessionKey := strings.Split(key, "-")[2]
			failures, err := strconv.Atoi(result)
			if err != nil || region.ServiceLog.SessionKey != sessionKey {
				continue
			}
			region.Failures = failures
		case strings.Contains(key, failureKey):
			region.Failure = result == "true"
		}
	}

	return nil
}

func geAppKeyFromLog(key string) string {
	split := strings.Split(key, "-")

	chain := split[0]
	chain = strings.ReplaceAll(chain, "{", "")
	chain = strings.ReplaceAll(chain, "}", "")
	return split[1] + "-" + chain
}

func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	snapCherryPickerData := &snapCherryPicker{}
	snapCherryPickerData.regionsData = make(map[string]*regionData)

	err := snapCherryPickerData.init(ctx)
	if err != nil {
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	err = snapCherryPickerData.snapCherryPickerData(ctx)
	if err != nil {
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(http.StatusOK, map[string]interface{}{
		"ok": true,
	}), err
}

func main() {
	snapCherryPickerData := &snapCherryPicker{}
	snapCherryPickerData.regionsData = make(map[string]*regionData)
	ctx := context.TODO()

	err := snapCherryPickerData.init(ctx)
	if err != nil {
		fmt.Println(err)
		// return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	err = snapCherryPickerData.snapCherryPickerData(ctx)
	if err != nil {
		fmt.Println(err)
		// return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	// lambda.Start(LambdaHandler)
}
