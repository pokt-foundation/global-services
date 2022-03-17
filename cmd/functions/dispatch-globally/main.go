package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Pocket/global-dispatcher/common/application"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"golang.org/x/sync/semaphore"
)

var (
	ErrMaxDispatchErrorsExceeded = errors.New("exceeded maximun allowance of dispatcher errors")

	rpcURL                      = environment.GetString("RPC_URL", "")
	dispatchURLs                = strings.Split(environment.GetString("DISPATCH_URLS", ""), ",")
	redisConnectionStrings      = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", ""), ",")
	mongoConnectionString       = environment.GetString("MONGODB_CONNECTION_STRING", "")
	mongoDatabase               = environment.GetString("MONGODB_DATABASE", "")
	cacheTTL                    = environment.GetInt64("CACHE_TTL", 3600)
	dispatchConcurrency         = environment.GetInt64("DISPATCH_CONCURRENCY", 200)
	maxDispatchersErrorsAllowed = environment.GetInt64("MAX_DISPATCHER_ERRORS_ALLOWED", 2000)
	dispatchGigastake           = environment.GetBool("DISPATCH_GIGASTAKE", false)

	headers = map[string]string{
		"Content-Type": "application/json",
	}
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration
type Response events.APIGatewayProxyResponse

// Handler is our lambda handler invoked by the `lambda.Start` function call
func LambdaHandler(ctx context.Context) (Response, error) {
	var buf bytes.Buffer
	var body []byte
	var encodeErr error
	var statusCode int

	failedDispatcherCalls, err := DispatchSessions(ctx)
	if err != nil {
		statusCode = http.StatusInternalServerError
		body, encodeErr = json.Marshal(map[string]interface{}{
			"ok":                    false,
			"error":                 err.Error(),
			"failedDispatcherCalls": failedDispatcherCalls,
		})
	} else {
		statusCode = http.StatusOK
		body, encodeErr = json.Marshal(map[string]interface{}{
			"ok":                    true,
			"failedDispatcherCalls": failedDispatcherCalls,
		})
	}

	if encodeErr != nil {
		return Response{StatusCode: http.StatusNotFound}, encodeErr
	}
	json.HTMLEscape(&buf, body)

	resp := Response{
		StatusCode:      statusCode,
		IsBase64Encoded: false,
		Body:            buf.String(),
		Headers:         headers,
	}

	return resp, err
}

func DispatchSessions(ctx context.Context) (uint32, error) {
	db, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return 0, errors.New("error connecting to mongo: " + err.Error())
	}

	commitHash := ""
	// TODO: Consumers have badly configured the commithash prefix and right now they don't use any kind of prefix
	// on their cache keys, uncomment when is fixed
	// commitHash, err := gateway.GetGatewayCommitHash()
	// if err != nil {
	// 	return 0, errors.New("error obtaining commit hash: " + err.Error())
	// }

	cacheClients, err := cache.GetCacheClients(redisConnectionStrings, commitHash)
	if err != nil {
		return 0, errors.New("error connecting to redis: " + err.Error())
	}

	pocketClient, err := pocket.NewPocketClient(rpcURL, dispatchURLs)
	if err != nil {
		return 0, errors.New("error obtaining a pocket client: " + err.Error())
	}

	blockHeight, err := pocketClient.GetBlockHeight()
	if err != nil {
		return 0, err
	}

	apps, err := application.GetAllStakedApplicationsOnDB(ctx, dispatchGigastake, db, pocketClient)
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

			go func(publicKey, chain string) {
				defer sem.Release(1)
				defer wg.Done()

				cacheKey := getSessionCacheKey(publicKey, chain, commitHash)

				shouldDispatch := ShouldDispatch(ctx, cacheClients, blockHeight, cacheKey)
				if !shouldDispatch {
					return
				}

				session, err := pocketClient.DispatchSession(pocket.DispatchInput{
					AppPublicKey: publicKey,
					Chain:        chain,
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

	return failedDispatcherCalls, nil
}

func ShouldDispatch(ctx context.Context, cacheClients []*cache.Redis, blockHeight int, cacheKey string) bool {
	rawSession, err := cacheClients[rand.Intn(len(cacheClients))].Client.Get(ctx, cacheKey).Result()
	if err != nil || rawSession == "" {
		return true
	}

	var cachedSession pocket.SessionCamelCase
	if err := json.Unmarshal([]byte(rawSession), &cachedSession); err != nil {
		return true
	}

	return cachedSession.BlockHeight < blockHeight
}

func getSessionCacheKey(publicKey, chain, commitHash string) string {
	return fmt.Sprintf("%ssession-cached-%s-%s", commitHash, publicKey, chain)
}

func main() {
	lambda.Start(LambdaHandler)
}
