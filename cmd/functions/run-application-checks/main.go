package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/Pocket/global-dispatcher/common/apigateway"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/common/gateway"
	"github.com/Pocket/global-dispatcher/common/gateway/models"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/Pocket/global-dispatcher/lib/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/pokt-foundation/pocket-go/pkg/client"
	"github.com/pokt-foundation/pocket-go/pkg/provider"
	"github.com/pokt-foundation/pocket-go/pkg/relayer"
	"github.com/pokt-foundation/pocket-go/pkg/signer"
	"golang.org/x/sync/semaphore"

	logger "github.com/Pocket/global-dispatcher/lib/logger"
	log "github.com/sirupsen/logrus"
)

var (
	ErrNoCacheClientProvided = errors.New("no cache clients were provided")

	rpcURL                 = environment.GetString("RPC_URL", "")
	dispatchURLs           = strings.Split(environment.GetString("DISPATCH_URLS", ""), ",")
	redisConnectionStrings = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", ""), ",")
	isRedisCluster         = environment.GetBool("IS_REDIS_CLUSTER", false)
	mongoConnectionString  = environment.GetString("MONGODB_CONNECTION_STRING", "")
	mongoDatabase          = environment.GetString("MONGODB_DATABASE", "")
	cacheTTL               = environment.GetInt64("CACHE_TTL", 3600)
	dispatchConcurrency    = environment.GetInt64("DISPATCH_CONCURRENCY", 2)
	dispatchGigastake      = environment.GetBool("DISPATCH_GIGASTAKE", true)
	maxClientsCacheCheck   = environment.GetInt64("MAX_CLIENTS_CACHE_CHECK", 3)
	appPrivateKey          = environment.GetString("APPLICATION_PRIVATE_KEY", "")
	defaultSyncAllowance   = environment.GetInt64("DEFAULT_SYNC_ALLOWANCE", 5)
	altruistTrustThreshold = environment.GetFloat64("ALTRUIST_TRUST_THRESHOLD", 0.5)
	syncCheckKeyPrefix     = environment.GetString("SYNC_CHECK_KEY_PREFIX", "")
	chainheckKeyPrefix     = environment.GetString("CHAIN_CHECK_KEY_PREFIX", "")

	headers = map[string]string{
		"Content-Type": "application/json",
	}
)

type ApplicationChecks struct {
	Caches      []*cache.Redis
	Provider    *provider.JSONRPCProvider
	Relayer     *relayer.PocketRelayer
	BlockHeight int
	CommitHash  string
	Blockchains map[string]*models.Blockchain
	RequestID   string
}

func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	var statusCode int

	err := RunApplicationChecks(ctx, lc.AwsRequestID)
	if err != nil {
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(statusCode, map[string]interface{}{
		"ok": true,
	}), err
}

func RunApplicationChecks(ctx context.Context, requestID string) error {
	if len(redisConnectionStrings) <= 0 {
		return ErrNoCacheClientProvided
	}

	db, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return errors.New("error connecting to mongo: " + err.Error())
	}

	caches, err := cache.ConnectoCacheClients(redisConnectionStrings, "", isRedisCluster)
	if err != nil {
		return errors.New("error connecting to redis: " + err.Error())
	}

	rpcPovider := provider.NewJSONRPCProvider(rpcURL, dispatchURLs, client.NewDefaultClient())

	wallet, err := signer.NewWalletFromPrivatekey(appPrivateKey)
	if err != nil {
		return errors.New("error creating wallet: " + err.Error())
	}

	pocketRelayer := relayer.NewPocketRelayer(wallet, rpcPovider)

	blockHeight, err := rpcPovider.GetBlockHeight()
	if err != nil {
		return err
	}

	ntApps, dbApps, err := gateway.GetStakedApplicationsOnDB(ctx, dispatchGigastake, db, rpcPovider)
	if err != nil {
		return errors.New("error obtaining staked apps on db: " + err.Error())
	}

	blockchainsDB, err := db.GetBlockchains(ctx)
	if err != nil {
		return errors.New("error obtaining blockchains: " + err.Error())
	}

	blockchains := utils.SliceToMappedStruct(blockchainsDB, func(bc *models.Blockchain) string {
		return bc.ID
	})

	appChecks := ApplicationChecks{
		Caches:      caches,
		Provider:    rpcPovider,
		Relayer:     pocketRelayer,
		BlockHeight: blockHeight,
		RequestID:   requestID,
	}

	var sem = semaphore.NewWeighted(dispatchConcurrency)
	var wg sync.WaitGroup

	for index, app := range ntApps {
		for _, chain := range app.Chains {
			sem.Acquire(ctx, 1)
			wg.Add(1)

			go func(publicKey, ch string, idx int) {
				defer sem.Release(1)
				defer wg.Done()

				dbApp := dbApps[idx]

				session, err := appChecks.GetSession(ctx, publicKey, ch)
				if err != nil {
					logger.Log.WithFields(log.Fields{
						"appPublicKey": publicKey,
						"chain":        ch,
						"error":        err.Error(),
					}).Error("error dispatching: " + err.Error())
					return
				}

				blockchain := *blockchains[ch]

				pocketAAT := provider.PocketAAT{
					AppPubKey:    dbApp.GatewayAAT.ApplicationPublicKey,
					ClientPubKey: dbApp.GatewayAAT.ClientPublicKey,
					Version:      dbApp.GatewayAAT.Version,
					Signature:    dbApp.GatewayAAT.ApplicationSignature,
				}

				wg.Add(1)
				go func() {
					defer wg.Done()
					appChecks.ChainCheck(ctx, pocket.ChainCheckOptions{
						Session:    *session,
						Blockchain: blockchain.ID,
						Data:       blockchain.ChainIDCheck,
						ChainID:    blockchain.ChainID,
						Path:       blockchain.Path,
						PocketAAT:  pocketAAT,
						RequestID:  requestID,
					}, blockchain, caches)
				}()

				wg.Add(1)
				go func() {
					defer wg.Done()
					appChecks.SyncCheck(ctx, pocket.SyncCheckOptions{
						Session:          *session,
						PocketAAT:        pocketAAT,
						SyncCheckOptions: blockchain.SyncCheckOptions,
						Path:             blockchain.Path,
						AltruistURL:      blockchain.Altruist,
						Blockchain:       blockchain.ID,
						RequestID:        requestID,
					}, blockchain, caches)
				}()
			}(app.PublicKey, chain, index)
		}
	}

	wg.Wait()

	err = cache.CloseConnections(caches)
	if err != nil {
		return err
	}

	return nil
}

func (ac *ApplicationChecks) GetSession(ctx context.Context, publicKey, chain string) (*provider.Session, error) {
	_, cachedSession := gateway.ShouldDispatch(ctx, ac.Caches, ac.BlockHeight,
		gateway.GetSessionCacheKey(publicKey, chain, ac.CommitHash), int(maxClientsCacheCheck))

	if cachedSession != nil {
		return cachedSession, nil
	}

	dispatch, err := ac.Provider.Dispatch(publicKey, chain, nil)
	if err != nil {
		return nil, err
	}

	return dispatch.Session, nil
}

func (ac *ApplicationChecks) ChainCheck(ctx context.Context, options pocket.ChainCheckOptions, blockchain models.Blockchain, caches []*cache.Redis) []string {
	if blockchain.ChainIDCheck == "" {
		return []string{}
	}

	nodes := (&pocket.ChainChecker{
		Relayer:    ac.Relayer,
		CommitHash: ac.CommitHash,
	}).Check(options)

	cacheKey := ac.CommitHash + syncCheckKeyPrefix + options.Session.Key

	if err := cache.WriteJSONToCaches(ctx, caches, cacheKey, nodes, uint(cacheTTL)); err != nil {
		logger.Log.WithFields(log.Fields{
			"appPublicKey": options.Session.Header.AppPublicKey,
			"chain":        options.Session.Header.Chain,
			"error":        err.Error(),
			"requestID":    ac.RequestID,
		}).Error("sync check: error writing to cache: " + err.Error())
	}

	return nodes
}

func (ac *ApplicationChecks) SyncCheck(ctx context.Context, options pocket.SyncCheckOptions, blockchain models.Blockchain, caches []*cache.Redis) []string {
	if blockchain.SyncCheckOptions.Body == "" {
		return []string{}
	}

	nodes := (&pocket.SyncChecker{
		Relayer:                ac.Relayer,
		CommitHash:             ac.CommitHash,
		DefaultSyncAllowance:   int(defaultSyncAllowance),
		AltruistTrustThreshold: float32(altruistTrustThreshold),
	}).Check(options)

	cacheKey := ac.CommitHash + chainheckKeyPrefix + options.Session.Key

	if err := cache.WriteJSONToCaches(ctx, caches, cacheKey, nodes, uint(cacheTTL)); err != nil {
		logger.Log.WithFields(log.Fields{
			"appPublicKey": options.Session.Header.AppPublicKey,
			"chain":        options.Session.Header.Chain,
			"error":        err.Error(),
			"requestID":    ac.RequestID,
		}).Error("chain check: error writing to cache: " + err.Error())
	}

	return nodes
}

func main() {
	lambda.Start(LambdaHandler)
}
