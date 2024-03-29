package base

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Pocket/global-services/shared/cache"
	"github.com/Pocket/global-services/shared/database"
	dbclient "github.com/pokt-foundation/db-client/client"
	"github.com/pokt-foundation/utils-go/environment"

	shared "github.com/Pocket/global-services/shared/error"
	"github.com/Pocket/global-services/shared/gateway"
	"github.com/Pocket/global-services/shared/pocket"
	"github.com/pokt-foundation/pocket-go/provider"
	"golang.org/x/sync/semaphore"

	logger "github.com/Pocket/global-services/shared/logger"
	log "github.com/sirupsen/logrus"
)

var (
	errMaxDispatchErrorsExceeded = errors.New("exceeded maximum allowance of dispatcher errors")
	errLessThanMinimumNodes      = errors.New("there are less than the minimum session nodes found")

	rpcURL                      = environment.MustGetString("RPC_URL")
	dispatchURLs                = strings.Split(environment.MustGetString("DISPATCH_URLS"), ",")
	redisConnectionStrings      = strings.Split(environment.MustGetString("REDIS_CONNECTION_STRINGS"), ",")
	isRedisCluster              = environment.GetBool("IS_REDIS_CLUSTER", false)
	cacheTTL                    = environment.GetInt64("CACHE_TTL", 3600)
	dispatchConcurrency         = environment.GetInt64("DISPATCH_CONCURRENCY", 200)
	maxDispatchersErrorsAllowed = environment.GetInt64("MAX_DISPATCHER_ERRORS_ALLOWED", 2000)
	maxClientsCacheCheck        = environment.GetInt64("MAX_CLIENTS_CACHE_CHECK", 3)
	cacheBatchSize              = environment.GetInt64("CACHE_BATCH_SIZE", 100)
	phdBaseURL                  = environment.MustGetString("PHD_BASE_URL")
	phdAPIKey                   = environment.MustGetString("PHD_API_KEY")
)

// DispatchSessions obtains applications from the database, asserts they're staked
// and dispatch the sessions of the chains from the applications, writing the results
// to the cache clients provided while also  reporting any failure from the dispatchers.
func DispatchSessions(ctx context.Context, requestID string) (uint32, error) {
	if len(redisConnectionStrings) <= 0 {
		return 0, shared.ErrNoCacheClientProvided
	}

	dbClient, err := database.NewPHDClient(dbclient.Config{
		BaseURL: phdBaseURL,
		APIKey:  phdAPIKey,
		Version: dbclient.V1,
	})
	if err != nil {
		return 0, errors.New("error validating phd config: " + err.Error())
	}

	caches, err := cache.ConnectToCacheClients(ctx, redisConnectionStrings, "", isRedisCluster)
	if err != nil {
		return 0, errors.New("error connecting to redis: " + err.Error())
	}

	rpcProvider := provider.NewProvider(rpcURL, dispatchURLs)

	blockHeight, err := rpcProvider.GetBlockHeight()
	if err != nil {
		return 0, errors.New("error obtaining block height: " + err.Error())
	}

	apps, _, err := dbClient.GetStakedApplications(ctx, rpcProvider)
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
					// Such sessions cannot be dispatched so not an actual error
					if strings.Contains(err.Error(), errLessThanMinimumNodes.Error()) {
						return
					}

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
		return failedDispatcherCalls, errMaxDispatchErrorsExceeded
	}

	err = cache.CloseConnections(caches)
	if err != nil {
		return 0, err
	}

	return failedDispatcherCalls, nil
}
