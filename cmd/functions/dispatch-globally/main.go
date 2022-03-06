package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Pocket/global-dispatcher/common/application"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/common/gateway"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

var (
	rpcURL                 = environment.GetString("RPC_URL", "")
	dispatchURLs           = strings.Split(environment.GetString("DISPATCH_URLS", ""), ",")
	redisConnectionStrings = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", ""), ",")
	mongoConnectionString  = environment.GetString("MONGODB_CONNECTION_STRING", "")
	mongoDatabase          = environment.GetString("MONGODB_DATABASE", "")
	cacheTTL               = environment.GetInt64("CACHE_TTL", 360)
	dispatchConcurrency    = environment.GetInt64("DISPATCH_CONCURRENCY", 200)
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration
type Response events.APIGatewayProxyResponse

// Handler is our lambda handler invoked by the `lambda.Start` function call
func LambdaHandler(ctx context.Context) (Response, error) {
	var buf bytes.Buffer
	ok := true

	failedDispatcherCalls, err := Handler()

	if err != nil {
		ok = false
		fmt.Println(err)
	}

	body, err := json.Marshal(map[string]interface{}{
		"ok":                    ok,
		"failedDispatcherCalls": failedDispatcherCalls,
	})
	if err != nil {
		return Response{StatusCode: 404}, err
	}
	json.HTMLEscape(&buf, body)

	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            buf.String(),
		Headers: map[string]string{
			"Content-Type":           "application/json",
			"X-MyCompany-Func-Reply": "dispatch-globally",
		},
	}

	return resp, nil
}

func Handler() (uint32, error) {
	ctx := context.Background()

	db, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return 0, err
	}
	pocketClient, err := pocket.NewPocketClient(rpcURL, dispatchURLs)
	if err != nil {
		return 0, err
	}

	commitHash, err := gateway.GetGatewayCommitHash()
	if err != nil {
		return 0, err
	}

	cacheClients, err := cache.GetCacheClients(redisConnectionStrings, commitHash)
	if err != nil {
		return 0, err
	}

	apps, err := application.GetAllStakedApplicationsOnDB(ctx, db, *pocketClient)
	if err != nil {
		return 0, err
	}

	var failedDispatcherCalls uint32
	var sem = semaphore.NewWeighted(dispatchConcurrency)

	for _, app := range apps {
		for _, chain := range app.Chains {
			sem.Acquire(ctx, 1)
			go func(publicKey, chain string) {
				defer sem.Release(1)
				DispatchAndCacheSession(pocketClient, cacheClients, commitHash, publicKey, chain, &failedDispatcherCalls)
			}(app.PublicKey, chain)
		}
	}

	return failedDispatcherCalls, nil
}

func DispatchAndCacheSession(
	client *pocket.PocketJsonRpcClient, cacheClients []*cache.Redis, commitHash string,
	publicKey string, chain string, counter *uint32) {
	session, err := client.DispatchSession(pocket.DispatchInput{
		AppPublicKey: publicKey,
		Chain:        chain,
	})
	if err != nil {
		atomic.AddUint32(counter, 1)
		fmt.Println("error dispatching:", err)
		return
	}

	cacheKey := fmt.Sprintf("%ssession-cached-%s-%s", commitHash, publicKey, chain)
	err = WriteJSONToCaches(cacheClients, cacheKey, session, uint(cacheTTL))
	if err != nil {
		atomic.AddUint32(counter, 1)
		fmt.Println("error writing to cache:", err)
	}
}

func WriteJSONToCaches(cacheClients []*cache.Redis, key string, value interface{}, TTLSeconds uint) error {
	var g errgroup.Group
	for _, cacheClient := range cacheClients {
		func(ch *cache.Redis) {
			g.Go(func() error {
				ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
				defer cancel()

				return ch.SetJSON(ctx, key, value, TTLSeconds)
			})
		}(cacheClient)
	}

	return g.Wait()
}

func main() {
	lambda.Start(LambdaHandler)
}
