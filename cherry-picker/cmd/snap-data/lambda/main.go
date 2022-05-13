package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/exp/slices"
	"golang.org/x/sync/semaphore"

	cpicker "github.com/Pocket/global-services/cherry-picker"
	db "github.com/Pocket/global-services/cherry-picker/database"
	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/cache"
	"github.com/Pocket/global-services/shared/database"
	"github.com/Pocket/global-services/shared/environment"
	shared "github.com/Pocket/global-services/shared/error"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	poktutils "github.com/pokt-foundation/pocket-go/utils"
)

var (
	cherryPickerConnections = strings.Split(environment.GetString("CHERRY_PICKER_CONNECTIONS	", "postgres://postgres:postgres@localhost:5432/postgres"), ",")
	redisConnectionStrings  = environment.GetString("REDIS_REGION_CONNECTION_STRINGS", "{\"localhost\": \"localhost:6379\"}")
	isRedisCluster          = environment.GetBool("IS_REDIS_CLUSTER", false)
	concurrency             = environment.GetInt64("CONCURRENCY", 1)
	successKey              = environment.GetString("SUCCESS_KEY", "success-hits")
	failuresKey             = environment.GetString("SUCCESS_KEY", "failure-hits")
	failureKey              = environment.GetString("SUCCESS_KEY", "failure")
	sessionTableName        = environment.GetString("SESSION_TABLE_NAME", "cherry_picker_session")
	sessionRegionTableName  = environment.GetString("SESSION_REGION_TABLE_NAME", "cherry_picker_session_region")
)

type ApplicationData struct {
	ServiceLog             cpicker.ServiceLog
	Address                string
	Successes              int
	Failures               int
	Failure                bool
	PublicKey              string
	Chain                  string
	MedianSuccessLatency   float32
	WeightedSuccessLatency float32
}

type Region struct {
	Cache   *cache.Redis
	Name    string
	AppData map[string]*ApplicationData
}

type SessionKey struct {
	PublicKey  string
	Chain      string
	SessionKey string
}

type SnapCherryPicker struct {
	Regions   map[string]*Region
	Caches    []*cache.Redis
	Stores    []cpicker.CherryPickerStore
	RequestID string
}

func (sn *SnapCherryPicker) init(ctx context.Context) error {
	if err := sn.initRegionCaches(ctx); err != nil {
		return err
	}

	for _, connString := range cherryPickerConnections {
		connection, err := db.NewCherryPickerPostgresFromConnectionString(ctx, &database.PostgresOptions{
			Connection:  connString,
			MinPoolSize: 10,
			MaxPoolSize: 10,
		}, sessionTableName, sessionRegionTableName)
		if err != nil {
			return err
		}
		sn.Stores = append(sn.Stores, connection)
	}

	return nil
}

func (sn *SnapCherryPicker) initRegionCaches(ctx context.Context) error {
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
		sn.Regions[region] = &Region{
			Cache:   ch,
			Name:    region,
			AppData: make(map[string]*ApplicationData),
		}
		sn.Caches = append(sn.Caches, ch)
	}

	return nil
}

func (sn *SnapCherryPicker) snapCherryPickerData(ctx context.Context) error {
	if err := sn.getAppsRegionsData(ctx); err != nil {
		return err
	}
	sn.saveToStore(ctx)
	return nil
}

func (sn *SnapCherryPicker) saveToStore(ctx context.Context) {
	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(concurrency)

	sessionsInStore := map[string]*SessionKey{}
	for _, region := range sn.Regions {
		for _, application := range region.AppData {
			wg.Add(1)
			sem.Acquire(ctx, 1)

			go func(rg *Region, app *ApplicationData) {
				defer wg.Done()
				defer sem.Release(1)

				sessionInStoreKey := fmt.Sprintf("%s-%s-%s", app.PublicKey, app.Chain, app.ServiceLog.SessionKey)
				// v, ok := sessionsInStore[sessionInStoreKey]
				if _, ok := sessionsInStore[sessionInStoreKey]; !ok {
					if err := sn.createSessionIfDoesntExist(ctx, app); err != nil {
						// TODO: LOG
						return
					}
					sessionsInStore[sessionInStoreKey] = &SessionKey{
						PublicKey:  app.PublicKey,
						Chain:      app.Chain,
						SessionKey: app.ServiceLog.SessionKey,
					}
				}

				if _, err := sn.createOrUpdateRegion(ctx, rg.Name, app); err != nil {
					// TODO: LOG
				}
			}(region, application)
		}
	}
	wg.Wait()

	sn.aggregateRegionData(ctx, sessionsInStore)
}

func (sn *SnapCherryPicker) aggregateRegionData(ctx context.Context, sessionsInStore map[string]*SessionKey) {
	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(concurrency)

	for _, session := range sessionsInStore {
		wg.Add(1)
		sem.Acquire(ctx, 1)

		go func(sess *SessionKey) {
			defer wg.Done()
			defer sem.Release(1)

			err := utils.RunFnOnSlice(sn.Stores, func(st cpicker.CherryPickerStore) error {
				regions, err := st.GetSessionRegions(ctx, sess.PublicKey, sess.Chain, sess.SessionKey)
				if err != nil {
					return err
				}
				session := &cpicker.UpdateSession{
					PublicKey:  sess.PublicKey,
					Chain:      sess.Chain,
					SessionKey: sess.SessionKey,
				}
				avgSuccessTimes := []float32{}

				for _, region := range regions {
					session.TotalSuccess += region.TotalSuccess
					session.TotalFailure += region.TotalFailure
					session.Failure = session.Failure || region.Failure
					avgSuccessTimes = append(avgSuccessTimes, region.MedianSuccessLatency...)
				}
				session.AverageSuccessTime = utils.AvgOfSlice(avgSuccessTimes)

				_, err = st.UpdateSession(ctx, session)
				return err
			})

			if err != nil {
				// TODO: LOG
			}
		}(session)
	}
}

func (sn *SnapCherryPicker) createOrUpdateRegion(ctx context.Context, regionName string,
	app *ApplicationData) (*cpicker.Region, error) {
	region := &cpicker.Region{
		PublicKey:                 app.PublicKey,
		Chain:                     app.Chain,
		SessionKey:                app.ServiceLog.SessionKey,
		Address:                   app.Address,
		Region:                    regionName,
		SessionHeight:             app.ServiceLog.SessionHeight,
		TotalSuccess:              app.Successes,
		TotalFailure:              app.Failures,
		MedianSuccessLatency:      []float32{app.MedianSuccessLatency},
		WeightedSuccessLatency:    []float32{app.WeightedSuccessLatency},
		AvgSuccessLatency:         app.MedianSuccessLatency,
		AvgWeightedSuccessLatency: app.WeightedSuccessLatency,
		Failure:                   app.Failure,
	}

	err := utils.RunFnOnSlice(sn.Stores, func(st cpicker.CherryPickerStore) error {
		storeRegion, err := st.GetRegion(ctx, app.PublicKey, app.Chain, app.ServiceLog.SessionKey, regionName)
		if err != nil {
			if err != db.ErrNotFound {
				return err
			}
			return st.CreateRegion(ctx, region)
		}

		updateRegion := &cpicker.UpdateRegion{
			PublicKey:              app.PublicKey,
			Chain:                  app.Chain,
			SessionKey:             app.ServiceLog.SessionKey,
			Region:                 regionName,
			TotalSuccess:           app.Successes,
			TotalFailure:           app.Failures,
			MedianSuccessLatency:   app.MedianSuccessLatency,
			WeightedSuccessLatency: app.WeightedSuccessLatency,
			AvgSuccessLatency: float32(utils.AvgOfSlice(
				append(storeRegion.MedianSuccessLatency, app.MedianSuccessLatency))),
			AvgWeightedSuccessLatency: float32(utils.AvgOfSlice(
				append(storeRegion.WeightedSuccessLatency, app.WeightedSuccessLatency))),
			Failure: app.Failure || storeRegion.Failure,
		}

		region, err = st.UpdateRegion(ctx, updateRegion)
		return err
	})

	return region, err
}

func (sn *SnapCherryPicker) createSessionIfDoesntExist(ctx context.Context, app *ApplicationData) error {
	return utils.RunFnOnSlice(sn.Stores, func(st cpicker.CherryPickerStore) error {
		_, err := st.GetSession(ctx, app.PublicKey, app.Chain, app.ServiceLog.SessionKey)
		if err == db.ErrNotFound {
			return st.CreateSession(ctx, &cpicker.Session{
				PublicKey:     app.PublicKey,
				Chain:         app.Chain,
				SessionKey:    app.ServiceLog.SessionKey,
				SessionHeight: app.ServiceLog.SessionHeight,
				Address:       app.Address,
			})
		}
		return err
	})
}

func (sn *SnapCherryPicker) getAppsRegionsData(ctx context.Context) error {
	return utils.RunFnOnSlice(sn.Caches, func(cl *cache.Redis) error {
		if err := sn.getServiceLogData(ctx, cl); err != nil {
			return err
		}
		return sn.getSuccessAndFailureData(ctx, cl)
	})
}

func (sn *SnapCherryPicker) getServiceLogData(ctx context.Context, cl *cache.Redis) error {
	serviceLogKeys, err := cl.Client.Keys(ctx, "*service*").Result()
	if err != nil {
		return err
	}

	results, err := cl.Client.MGet(ctx, serviceLogKeys...).Result()
	if err != nil {
		return err
	}
	for idx, rawServiceLog := range results {
		publicKey, chain := getPublicKeyAndChainFromLog(serviceLogKeys[idx])
		appDataKey := publicKey + "-" + chain
		sn.Regions[cl.Name].AppData[appDataKey] = &ApplicationData{}
		appData := sn.Regions[cl.Name].AppData[appDataKey]
		appData.PublicKey = publicKey
		appData.Chain = chain
		address, _ := poktutils.GetAddressFromPublickey(publicKey)
		appData.Address = address

		if err := cache.UnMarshallJSONResult(rawServiceLog, nil, &appData.ServiceLog); err != nil {
			// TODO: Log errors
			continue
		}
		// TODO: Log errors

		medianSuccessLatency, _ := strconv.ParseFloat(appData.ServiceLog.MedianSuccessLatency, 32)
		appData.MedianSuccessLatency = float32(medianSuccessLatency)

		weightedSuccessLatency, _ := strconv.ParseFloat(appData.ServiceLog.WeightedSuccessLatency, 32)
		appData.WeightedSuccessLatency = float32(weightedSuccessLatency)
	}

	return nil
}

func (sn *SnapCherryPicker) getSuccessAndFailureData(ctx context.Context, cl *cache.Redis) error {
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
		publicKey, chain := getPublicKeyAndChainFromLog(key)
		appDataKey := publicKey + "-" + chain
		region := sn.Regions[cl.Name].AppData[appDataKey]
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

func getPublicKeyAndChainFromLog(key string) (string, string) {
	split := strings.Split(key, "-")

	publicKey := split[1]
	chain := split[0]
	// Chain key could have braces between to use the same slot on a redis cluster
	chain = strings.ReplaceAll(chain, "{", "")
	chain = strings.ReplaceAll(chain, "}", "")
	return publicKey, chain
}

func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	snapCherryPickerData := &SnapCherryPicker{
		RequestID: lc.AwsRequestID,
	}
	snapCherryPickerData.Regions = make(map[string]*Region)

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
	snapCherryPickerData := &SnapCherryPicker{
		RequestID: "1234",
	}
	snapCherryPickerData.Regions = make(map[string]*Region)
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
