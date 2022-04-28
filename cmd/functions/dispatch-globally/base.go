package base

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/common/gateway"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/pokt-foundation/pocket-go/pkg/provider"
	"golang.org/x/sync/semaphore"

	logger "github.com/Pocket/global-dispatcher/lib/logger"
	log "github.com/sirupsen/logrus"
)

var (
	ErrMaxDispatchErrorsExceeded = errors.New("exceeded maximun allowance of dispatcher errors")
	ErrNoCacheClientProvided     = errors.New("no cache clients were provided")

	rpcURL                      = environment.GetString("RPC_URL", "https://shared-use2.nodes.pokt.network:18081")
	dispatchURLs                = strings.Split(environment.GetString("DISPATCH_URLS", "https://dispatch-1.nodes.pokt.network:4201,https://dispatch-2.nodes.pokt.network:4202,https://dispatch-3.nodes.pokt.network:4203,https://dispatch-4.nodes.pokt.network:4204,https://dispatch-5.nodes.pokt.network:4205,https://dispatch-6.nodes.pokt.network:4206,https://dispatch-7.nodes.pokt.network:4207,https://dispatch-8.nodes.pokt.network:4208,https://dispatch-9.nodes.pokt.network:4209,https://dispatch-10.nodes.pokt.network:4210,https://dispatch-11.nodes.pokt.network:4211,https://dispatch-12.nodes.pokt.network:4212,https://dispatch-13.nodes.pokt.network:4213,https://dispatch-14.nodes.pokt.network:4214,https://dispatch-15.nodes.pokt.network:4215,https://dispatch-16.nodes.pokt.network:4216,https://dispatch-17.nodes.pokt.network:4217,https://dispatch-18.nodes.pokt.network:4218,https://dispatch-19.nodes.pokt.network:4219,https://dispatch-20.nodes.pokt.network:4220,https://dispatch-21.nodes.pokt.network:4221,https://dispatch-22.nodes.pokt.network:4222,https://dispatch-23.nodes.pokt.network:4223,https://dispatch-24.nodes.pokt.network:4224,https://dispatch-25.nodes.pokt.network:4225,https://dispatch-26.nodes.pokt.network:4226,https://dispatch-27.nodes.pokt.network:4227,https://dispatch-28.nodes.pokt.network:4228,https://dispatch-29.nodes.pokt.network:4229,https://dispatch-30.nodes.pokt.network:4230,https://dispatch-34.nodes.pokt.network:4234,https://dispatch-35.nodes.pokt.network:4235,https://dispatch-36.nodes.pokt.network:4236,https://dispatch-37.nodes.pokt.network:4237,https://dispatch-38.nodes.pokt.network:4238,https://dispatch-39.nodes.pokt.network:4239,https://dispatch-40.nodes.pokt.network:4240,https://dispatch-41.nodes.pokt.network:4241,https://dispatch-42.nodes.pokt.network:4242,https://dispatch-43.nodes.pokt.network:4243,https://dispatch-44.nodes.pokt.network:4244"), ",")
	redisConnectionStrings      = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", "localhost:6379"), ",")
	isRedisCluster              = environment.GetBool("IS_REDIS_CLUSTER", false)
	mongoConnectionString       = environment.GetString("MONGODB_CONNECTION_STRING", "")
	mongoDatabase               = environment.GetString("MONGODB_DATABASE", "gateway")
	cacheTTL                    = environment.GetInt64("CACHE_TTL", 3600)
	dispatchConcurrency         = environment.GetInt64("DISPATCH_CONCURRENCY", 200)
	maxDispatchersErrorsAllowed = environment.GetInt64("MAX_DISPATCHER_ERRORS_ALLOWED", 2000)
	maxClientsCacheCheck        = environment.GetInt64("MAX_CLIENTS_CACHE_CHECK", 3)
	cacheBatchSize              = environment.GetInt64("CACHE_BATCH_SIZE", 100)
)

// DispatchSessions obtains applications from the database, asserts they're staked
// and dispatch the sessions of the chains from the applications, writing the results
// to the cache clients provided while also  reporting any failure from the dispatchers.
func DispatchSessions(ctx context.Context, requestID string) (uint32, error) {
	if len(redisConnectionStrings) <= 0 {
		return 0, ErrNoCacheClientProvided
	}

	db, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return 0, errors.New("error connecting to mongo: " + err.Error())
	}

	caches, err := cache.ConnectoCacheClients(ctx, redisConnectionStrings, "", isRedisCluster)
	if err != nil {
		return 0, errors.New("error connecting to redis: " + err.Error())
	}

	rpcProvider := provider.NewJSONRPCProvider(rpcURL, dispatchURLs)

	blockHeight, err := rpcProvider.GetBlockHeight()
	if err != nil {
		return 0, errors.New("error obtaining block height: " + err.Error())
	}

	apps, _, err := gateway.GetGigastakedApplicationsOnDB(ctx, db, rpcProvider)
	if err != nil {
		return 0, errors.New("error obtaining staked apps on db: " + err.Error())
	}

	var cacheWg sync.WaitGroup
	cacheWg.Add(1)
	cacheBatch := cache.BatchWriter(ctx, &cache.BatchWriterOptions{
		Caches:    caches,
		BatchSize: int(cacheBatchSize),
		WaitGroup: &cacheWg,
		RequestID: requestID,
	})

	var failedDispatcherCalls uint32
	var sem = semaphore.NewWeighted(dispatchConcurrency)
	var wg sync.WaitGroup

	for _, app := range apps {
		for _, chain := range app.Chains {
			sem.Acquire(ctx, 1)
			wg.Add(1)

			go func(publicKey, ch string) {
				defer sem.Release(1)
				defer wg.Done()

				cacheKey := gateway.GetSessionCacheKey(publicKey, ch, "")

				shouldDispatch, _ := gateway.ShouldDispatch(ctx, caches, blockHeight, cacheKey, int(maxClientsCacheCheck))
				if !shouldDispatch {
					return
				}

				dispatch, err := rpcProvider.Dispatch(publicKey, ch, nil)
				if err != nil {
					atomic.AddUint32(&failedDispatcherCalls, 1)
					logger.Log.WithFields(log.Fields{
						"appPublicKey": publicKey,
						"chain":        ch,
						"error":        err.Error(),
						"requestID":    requestID,
					}).Error("error dispatching: " + err.Error())
					return
				}

				session := pocket.NewSessionCamelCase(dispatch.Session)
				// Embedding current block height within session so can be checked for cache
				session.BlockHeight = dispatch.BlockHeight

				marshalledSession, err := json.Marshal(session)
				if err != nil {
					logger.Log.WithFields(log.Fields{
						"error":        err.Error(),
						"requestID":    requestID,
						"blockchainID": ch,
						"sessionKey":   session.Key,
					}).Errorf("sync check: error marshalling nodes: %s", err.Error())
					return
				}

				cacheBatch <- &cache.Item{
					Key:   cacheKey,
					Value: marshalledSession,
					TTL:   time.Duration(cacheTTL) * time.Second,
				}
			}(app.PublicKey, chain)
		}
	}

	wg.Wait()

	close(cacheBatch)
	cacheWg.Wait()

	if failedDispatcherCalls > uint32(maxDispatchersErrorsAllowed) {
		return failedDispatcherCalls, ErrMaxDispatchErrorsExceeded
	}

	err = cache.CloseConnections(caches)
	if err != nil {
		return 0, err
	}

	return failedDispatcherCalls, nil
}
