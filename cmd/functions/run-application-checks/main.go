package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Pocket/global-dispatcher/common/apigateway"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/common/gateway"
	"github.com/Pocket/global-dispatcher/common/gateway/models"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/metrics"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/Pocket/global-dispatcher/lib/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/pokt-foundation/pocket-go/pkg/provider"
	"github.com/pokt-foundation/pocket-go/pkg/relayer"
	"github.com/pokt-foundation/pocket-go/pkg/signer"
	"golang.org/x/sync/semaphore"

	logger "github.com/Pocket/global-dispatcher/lib/logger"
	log "github.com/sirupsen/logrus"
)

var (
	errNoCacheClientProvided = errors.New("no cache clients were provided")

	rpcURL                 = environment.GetString("RPC_URL", "")
	dispatchURLs           = strings.Split(environment.GetString("DISPATCH_URLS", ""), ",")
	redisConnectionStrings = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", ""), ",")
	isRedisCluster         = environment.GetBool("IS_REDIS_CLUSTER", false)
	mongoConnectionString  = environment.GetString("MONGODB_CONNECTION_STRING", "")
	mongoDatabase          = environment.GetString("MONGODB_DATABASE", "")
	cacheTTL               = environment.GetInt64("CACHE_TTL", 300)
	dispatchConcurrency    = environment.GetInt64("DISPATCH_CONCURRENCY", 2)
	dispatchGigastake      = environment.GetBool("DISPATCH_GIGASTAKE", true)
	maxClientsCacheCheck   = environment.GetInt64("MAX_CLIENTS_CACHE_CHECK", 3)
	appPrivateKey          = environment.GetString("APPLICATION_PRIVATE_KEY", "")
	defaultSyncAllowance   = environment.GetInt64("DEFAULT_SYNC_ALLOWANCE", 5)
	altruistTrustThreshold = environment.GetFloat64("ALTRUIST_TRUST_THRESHOLD", 0.5)
	syncCheckKeyPrefix     = environment.GetString("SYNC_CHECK_KEY_PREFIX", "")
	chainheckKeyPrefix     = environment.GetString("CHAIN_CHECK_KEY_PREFIX", "")
	metricsConnection      = environment.GetString("METRICS_CONNECTION", "")
	minMetricsPoolSize     = environment.GetInt64("MIN_METRICS_POOL_SIZE", 5)
	maxMetricsPoolSize     = environment.GetInt64("MAX_METRICS_POOL_SIZE", 20)
	defaultTimeOut         = environment.GetInt64("DEFAULT_TIMEOUT", 8)
)

type applicationChecks struct {
	Caches          []*cache.Redis
	Provider        *provider.JSONRPCProvider
	Relayer         *relayer.PocketRelayer
	MetricsRecorder *metrics.Recorder
	BlockHeight     int
	CommitHash      string
	Blockchains     map[string]*models.Blockchain
	RequestID       string
	SyncChecker     *pocket.SyncChecker
	ChainChecker    *pocket.ChainChecker
}

func lambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	var statusCode int

	err := runApplicationChecks(ctx, lc.AwsRequestID)
	if err != nil {
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err

	}

	return *apigateway.NewJSONResponse(statusCode, map[string]interface{}{
		"ok": true,
	}), err
}

func runApplicationChecks(ctx context.Context, requestID string) error {
	if len(redisConnectionStrings) <= 0 {
		return errNoCacheClientProvided
	}

	db, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return errors.New("error connecting to mongo: " + err.Error())
	}

	metricsRecorder, err := metrics.NewMetricsRecorder(ctx, metricsConnection, int(minMetricsPoolSize), int(maxMetricsPoolSize))
	if err != nil {
		return errors.New("error connecting to metrics db: " + err.Error())
	}

	caches, err := cache.ConnectoCacheClients(ctx, redisConnectionStrings, "", isRedisCluster)
	if err != nil {
		return errors.New("error connecting to redis: " + err.Error())
	}

	rpcProvider := provider.NewJSONRPCProvider(rpcURL, dispatchURLs)
	rpcProvider.UpdateRequestConfig(0, time.Duration(defaultTimeOut)*time.Second)
	wallet, err := signer.NewWalletFromPrivatekey(appPrivateKey)
	if err != nil {
		return errors.New("error creating wallet: " + err.Error())
	}

	pocketRelayer := relayer.NewPocketRelayer(wallet, rpcProvider)

	blockHeight, err := rpcProvider.GetBlockHeight()
	if err != nil {
		return err
	}

	ntApps, dbApps, err := gateway.GetStakedApplicationsOnDB(ctx, dispatchGigastake, db, rpcProvider)
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

	appChecks := applicationChecks{
		Caches:          caches,
		Provider:        rpcProvider,
		Relayer:         pocketRelayer,
		MetricsRecorder: metricsRecorder,
		BlockHeight:     blockHeight,
		RequestID:       requestID,
		SyncChecker: &pocket.SyncChecker{
			Relayer:                pocketRelayer,
			DefaultSyncAllowance:   int(defaultSyncAllowance),
			AltruistTrustThreshold: float32(altruistTrustThreshold),
			MetricsRecorder:        metricsRecorder,
			RequestID:              requestID,
		},
		ChainChecker: &pocket.ChainChecker{
			Relayer:         pocketRelayer,
			MetricsRecorder: metricsRecorder,
			RequestID:       requestID,
		},
	}

	var sem = semaphore.NewWeighted(dispatchConcurrency)
	var wg sync.WaitGroup
	for index, app := range ntApps {
		for _, chain := range app.Chains {
			dbApp := dbApps[index]

			session, err := appChecks.getSession(ctx, app.PublicKey, chain)
			if err != nil {
				logger.Log.WithFields(log.Fields{
					"appPublicKey": app.PublicKey,
					"chain":        chain,
					"error":        err.Error(),
				}).Errorf("error dispatching: %s", err.Error())
				continue
			}

			blockchain := *blockchains[chain]

			pocketAAT := provider.PocketAAT{
				AppPubKey:    dbApp.GatewayAAT.ApplicationPublicKey,
				ClientPubKey: dbApp.GatewayAAT.ClientPublicKey,
				Version:      dbApp.GatewayAAT.Version,
				Signature:    dbApp.GatewayAAT.ApplicationSignature,
			}

			sem.Acquire(ctx, 1)
			wg.Add(1)
			go func() {
				defer sem.Release(1)
				defer wg.Done()
				appChecks.chainCheck(ctx, pocket.ChainCheckOptions{
					Session:    *session,
					Blockchain: blockchain.ID,
					Data:       blockchain.ChainIDCheck,
					ChainID:    blockchain.ChainID,
					Path:       blockchain.Path,
					PocketAAT:  pocketAAT,
				}, blockchain, caches)
			}()

			sem.Acquire(ctx, 1)
			wg.Add(1)
			go func() {
				defer sem.Release(1)
				defer wg.Done()
				appChecks.syncCheck(ctx, pocket.SyncCheckOptions{
					Session:          *session,
					PocketAAT:        pocketAAT,
					SyncCheckOptions: blockchain.SyncCheckOptions,
					AltruistURL:      blockchain.Altruist,
					Blockchain:       blockchain.ID,
				}, blockchain, caches)
			}()
		}
	}
	wg.Wait()

	metricsRecorder.Conn.Close()
	return cache.CloseConnections(caches)
}

func (ac *applicationChecks) getSession(ctx context.Context, publicKey, chain string) (*provider.Session, error) {
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

func (ac *applicationChecks) chainCheck(ctx context.Context, options pocket.ChainCheckOptions, blockchain models.Blockchain, caches []*cache.Redis) []string {
	if blockchain.ChainIDCheck == "" {
		return []string{}
	}

	nodes := ac.ChainChecker.Check(ctx, options)
	cacheKey := ac.CommitHash + syncCheckKeyPrefix + options.Session.Key
	ttl := cacheTTL
	if len(nodes) == 0 {
		ttl = 30
	}

	if err := cache.WriteJSONToCaches(ctx, caches, cacheKey, nodes, uint(ttl)); err != nil {
		logger.Log.WithFields(log.Fields{
			"chain":     options.Session.Header.Chain,
			"error":     err.Error(),
			"requestID": ac.RequestID,
		}).Errorf("sync check: error writing to cache: %s", err.Error())
	}

	return nodes
}

func (ac *applicationChecks) syncCheck(ctx context.Context, options pocket.SyncCheckOptions, blockchain models.Blockchain, caches []*cache.Redis) []string {
	if blockchain.SyncCheckOptions.Body == "" && blockchain.SyncCheckOptions.Path == "" {
		return []string{}
	}

	nodes := ac.SyncChecker.Check(ctx, options)
	cacheKey := ac.CommitHash + chainheckKeyPrefix + options.Session.Key
	ttl := cacheTTL
	if len(nodes) == 0 {
		ttl = 30
	}

	if err := cache.WriteJSONToCaches(ctx, caches, cacheKey, nodes, uint(ttl)); err != nil {
		logger.Log.WithFields(log.Fields{
			"chain":     options.Session.Header.Chain,
			"error":     err.Error(),
			"requestID": ac.RequestID,
		}).Errorf("sync check: error writing to cache: %s", err.Error())
	}

	if err := ac.eraseNodesFailureMark(ctx, options, nodes, caches); err != nil {
		logger.Log.WithFields(log.Fields{
			"appPublicKey": options.Session.Header.AppPublicKey,
			"chain":        options.Session.Header.Chain,
			"error":        err.Error(),
			"requestID":    ac.RequestID,
		}).Errorf("sync check: error erasing failure mark on cache: %s", err.Error())
	}

	return nodes
}

// eraseNodesFailureMark deletes the failure status on nodes on the api that were failing
// a significant amount of relays
func (ac *applicationChecks) eraseNodesFailureMark(ctx context.Context, options pocket.SyncCheckOptions, nodes []string, caches []*cache.Redis) error {
	getNodeFailureKey := func(blockchain, node string) string {
		return fmt.Sprintf("%s%s-%s-failure", ac.CommitHash, blockchain, node)
	}

	err := cache.RunFunctionOnAllClients(caches, func(ch *cache.Redis) error {
		pipe := ch.Client.Pipeline()
		for _, node := range nodes {
			pipe.Set(ctx, getNodeFailureKey(options.Blockchain, node), "false", 1*time.Hour)
		}
		_, err := pipe.Exec(ctx)
		return err
	})

	return err
}

func main() {
	lambda.Start(lambdaHandler)
}
