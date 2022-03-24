package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Pocket/global-dispatcher/common/apigateway"
	"github.com/Pocket/global-dispatcher/common/application"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/Pocket/global-dispatcher/lib/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"golang.org/x/sync/semaphore"
)

var (
	ErrMaxDispatchErrorsExceeded = errors.New("exceeded maximun allowance of dispatcher errors")
	ErrNoCacheClientProvided     = errors.New("no cache clients were provided")

	rpcURL                      = environment.GetString("RPC_URL", "")
	dispatchURLs                = strings.Split(environment.GetString("DISPATCH_URLS", ""), ",")
	redisConnectionStrings      = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", ""), ",")
	mongoConnectionString       = environment.GetString("MONGODB_CONNECTION_STRING", "")
	mongoDatabase               = environment.GetString("MONGODB_DATABASE", "")
	cacheTTL                    = environment.GetInt64("CACHE_TTL", 3600)
	dispatchConcurrency         = environment.GetInt64("DISPATCH_CONCURRENCY", 200)
	maxDispatchersErrorsAllowed = environment.GetInt64("MAX_DISPATCHER_ERRORS_ALLOWED", 2000)
	dispatchGigastake           = environment.GetBool("DISPATCH_GIGASTAKE", false)
	maxClientsCacheCheck        = environment.GetInt64("MAX_CLIENTS_CACHE_CHECK", 3)

	headers = map[string]string{
		"Content-Type": "application/json",
	}
)

// LambdaHandler manages the DispatchSession call to return as an APIGatewayProxyResponse
func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	var body []byte
	var statusCode int

	failedDispatcherCalls, err := DispatchSessions(ctx)
	if err != nil {
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	// Internal logging
	fmt.Printf("result: %s\n", string(body))

	return *apigateway.NewJSONResponse(statusCode, map[string]interface{}{
		"ok":                    true,
		"failedDispatcherCalls": failedDispatcherCalls,
	}), err
}

// DispatchSessions obtains applications from the database, asserts they're staked
// and dispatch the sessions of the chains from the applications, writing the results
// to the cache clients provided while also  reporting any failure from the dispatchers.
func DispatchSessions(ctx context.Context) (uint32, error) {
	if len(redisConnectionStrings) <= 0 {
		return 0, ErrNoCacheClientProvided
	}

	db, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return 0, errors.New("error connecting to mongo: " + err.Error())
	}

	commitHash := ""
	// TODO: Consumers have badly configured the commithash prefix and right now they
	// don't use any kind of prefix on their cache keys. Uncomment when is fixed.
	// commitHash, err := gateway.GetGatewayCommitHash()
	// if err != nil {
	// 	return 0, errors.New("error obtaining commit hash: " + err.Error())
	// }

	cacheClients, err := cache.ConnectoCacheClients(redisConnectionStrings, commitHash)
	if err != nil {
		return 0, errors.New("error connecting to redis: " + err.Error())
	}

	pocketClient, err := pocket.NewPocketClient(rpcURL, dispatchURLs)
	if err != nil {
		return 0, errors.New("error obtaining a pocket client: " + err.Error())
	}

	blockHeight, err := pocketClient.GetCurrentBlockHeight()
	if err != nil {
		return 0, err
	}

	apps, err := application.GetStakedApplicationsOnDB(ctx, dispatchGigastake, db, pocketClient)
	if err != nil {
		return 0, errors.New("error obtaining staked apps on db: " + err.Error())
	}

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

				cacheKey := getSessionCacheKey(publicKey, ch, commitHash)

				shouldDispatch := ShouldDispatch(ctx, cacheClients, blockHeight, cacheKey, int(maxClientsCacheCheck))
				if !shouldDispatch {
					return
				}

				session, err := pocketClient.DispatchSession(pocket.DispatchInput{
					AppPublicKey: publicKey,
					Chain:        ch,
				})
				if err != nil {
					atomic.AddUint32(&failedDispatcherCalls, 1)
					fmt.Println("error dispatching:", err)
					return
				}

				err = cache.WriteJSONToCaches(ctx, cacheClients, cacheKey, pocket.SessionCamelCase(*session), uint(cacheTTL))
				if err != nil {
					atomic.AddUint32(&failedDispatcherCalls, 1)
					fmt.Println("error writing to cache:", err)
				}
			}(app.PublicKey, chain)
		}
	}

	wg.Wait()

	if failedDispatcherCalls > uint32(maxDispatchersErrorsAllowed) {
		return failedDispatcherCalls, ErrMaxDispatchErrorsExceeded
	}

	err = cache.CloseConnections(cacheClients)
	if err != nil {
		return 0, err
	}

	return failedDispatcherCalls, nil
}

// ShouldDispatch checks N random cache clients and checks whether the session
// is available and up to date with the current block, fails if any of the
// clients fails the check.
func ShouldDispatch(ctx context.Context, cacheClients []*cache.Redis, blockHeight int, key string, maxClients int) bool {
	clientsToCheck := utils.Min(len(cacheClients), maxClients)
	clients := utils.Shuffle(cacheClients)[0:clientsToCheck]

	var wg sync.WaitGroup
	var cachedClients uint32

	for _, client := range clients {
		wg.Add(1)
		go func(cl *cache.Redis) {
			defer wg.Done()

			rawSession, err := cl.Client.Get(ctx, cl.KeyPrefix+key).Result()
			if err != nil || rawSession == "" {
				return
			}
			var cachedSession pocket.SessionCamelCase
			if err := json.Unmarshal([]byte(rawSession), &cachedSession); err != nil {
				return
			}
			if cachedSession.BlockHeight < blockHeight {
				return
			}

			atomic.AddUint32(&cachedClients, 1)
		}(client)
	}
	wg.Wait()

	return cachedClients != uint32(clientsToCheck)
}

func getSessionCacheKey(publicKey, chain, commitHash string) string {
	return fmt.Sprintf("%ssession-cached-%s-%s", commitHash, publicKey, chain)
}

func main() {
	lambda.Start(LambdaHandler)
}
