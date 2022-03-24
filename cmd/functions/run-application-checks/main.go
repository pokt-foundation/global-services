package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/Pocket/global-dispatcher/common/apigateway"
	"github.com/Pocket/global-dispatcher/common/application"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/common/gateway"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/pocket"
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

type ApplicationChecks struct {
	Caches      []*cache.Redis
	Pocket      pocket.PocketJsonRpcClient
	BlockHeight int
	CommitHash  string
}

func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	var body []byte
	var statusCode int

	err := RunApplicationChecks(ctx)
	if err != nil {
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	// Internal logging
	fmt.Printf("result: %s\n", string(body))

	return *apigateway.NewJSONResponse(statusCode, map[string]interface{}{
		"ok": true,
	}), err
}

func RunApplicationChecks(ctx context.Context) error {
	if len(redisConnectionStrings) <= 0 {
		return ErrNoCacheClientProvided
	}

	db, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return errors.New("error connecting to mongo: " + err.Error())
	}

	caches, err := cache.ConnectoCacheClients(redisConnectionStrings, "")
	if err != nil {
		return errors.New("error connecting to redis: " + err.Error())
	}

	pocketClient, err := pocket.NewPocketClient(rpcURL, dispatchURLs)
	if err != nil {
		return errors.New("error obtaining a pocket client: " + err.Error())
	}

	blockHeight, err := pocketClient.GetCurrentBlockHeight()
	if err != nil {
		return err
	}

	apps, err := application.GetStakedApplicationsOnDB(ctx, dispatchGigastake, db, pocketClient)
	if err != nil {
		return errors.New("error obtaining staked apps on db: " + err.Error())
	}

	appChecks := ApplicationChecks{
		Caches:      caches,
		Pocket:      *pocketClient,
		BlockHeight: blockHeight,
	}

	var sem = semaphore.NewWeighted(dispatchConcurrency)
	var wg sync.WaitGroup

	for _, app := range apps {
		for _, chain := range app.Chains {
			sem.Acquire(ctx, 1)
			wg.Add(1)

			go func(publicKey, ch string) {
				defer sem.Release(1)
				defer wg.Done()

				_, err := appChecks.GetSession(ctx, publicKey, ch)
				if err != nil {
					fmt.Println("error dispatching:", err)
				}
			}(app.PublicKey, chain)
		}
	}

	wg.Wait()

	err = cache.CloseConnections(caches)
	if err != nil {
		return err
	}

	return nil
}

func (ac *ApplicationChecks) GetSession(ctx context.Context, publicKey, chain string) (*pocket.SessionCamelCase, error) {
	_, cachedSession := gateway.ShouldDispatch(ctx, ac.Caches, ac.BlockHeight,
		gateway.GetSessionCacheKey(publicKey, chain, ac.CommitHash), int(maxClientsCacheCheck))

	if cachedSession != nil {
		return cachedSession, nil
	}

	session, err := ac.Pocket.DispatchSession(pocket.DispatchInput{
		AppPublicKey: publicKey,
		Chain:        chain,
	})
	if err != nil {
		return nil, err
	}
	sessionPtr := pocket.SessionCamelCase(*session)
	return &sessionPtr, nil
}

func main() {
	lambda.Start(LambdaHandler)
}
