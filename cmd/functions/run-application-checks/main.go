package main

import (
	"context"
	"encoding/json"
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
	cacheBatchSize         = environment.GetInt64("CACHE_BATCH_SIZE", 50)

	caches          []*cache.Redis
	metricsRecorder *metrics.Recorder
	db              *database.Mongo
	rpcProvider     *provider.JSONRPCProvider
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
	CacheBatch      chan *cache.Item
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
	var err error

	db, err = database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return errors.New("error connecting to mongo: " + err.Error())
	}

	metricsRecorder, err = metrics.NewMetricsRecorder(ctx, metricsConnection, int(minMetricsPoolSize), int(maxMetricsPoolSize))
	if err != nil {
		return errors.New("error connecting to metrics db: " + err.Error())
	}

	caches, err = cache.ConnectoCacheClients(ctx, redisConnectionStrings, "", isRedisCluster)
	if err != nil {
		return errors.New("error connecting to redis: " + err.Error())
	}

	rpcProvider = provider.NewJSONRPCProvider(rpcURL, dispatchURLs)
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

	var cacheWg sync.WaitGroup
	cacheWg.Add(1)
	cacheBatch := cache.BatchWriter(ctx, &cache.BatchWriterOptions{
		Caches:    caches,
		BatchSize: int(cacheBatchSize),
		WaitGroup: &cacheWg,
		RequestID: requestID,
	})

	appChecks := applicationChecks{
		Caches:          caches,
		Provider:        rpcProvider,
		Relayer:         pocketRelayer,
		MetricsRecorder: metricsRecorder,
		BlockHeight:     blockHeight,
		RequestID:       requestID,
		CacheBatch:      cacheBatch,
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

	ntApps = ntApps[15:17]

	var wg sync.WaitGroup
	var sem = semaphore.NewWeighted(dispatchConcurrency)
	for index, app := range ntApps {
		for _, chain := range app.Chains {
			wg.Add(1)
			go func(publicKey, ch string, idx int) {
				defer wg.Done()
				dbApp := dbApps[idx]

				session, err := appChecks.getSession(ctx, publicKey, ch)
				if err != nil {
					logger.Log.WithFields(log.Fields{
						"appPublicKey": publicKey,
						"chain":        ch,
						"error":        err.Error(),
					}).Errorf("error dispatching: %s", err.Error())
					return
				}

				blockchain := *blockchains[ch]

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
			}(app.PublicKey, chain, index)
		}
	}
	wg.Wait()

	close(cacheBatch)
	// Wait for the remaining items in the batch if any
	cacheWg.Wait()

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

	marshalledNodes, err := json.Marshal(nodes)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"error":        err.Error(),
			"requestID":    ac.RequestID,
			"blockchainID": blockchain,
			"sessionKey":   options.Session.Key,
		}).Errorf("sync check: error marshalling nodes: %s", err.Error())
		return nodes
	}

	ac.CacheBatch <- &cache.Item{
		Key:   cacheKey,
		Value: marshalledNodes,
		TTL:   time.Duration(ttl) * time.Second,
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

	marshalledNodes, err := json.Marshal(nodes)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"error":        err.Error(),
			"requestID":    ac.RequestID,
			"blockchainID": blockchain,
			"sessionKey":   options.Session.Key,
		}).Errorf("sync check: error marshalling nodes: %s", err.Error())
		return nodes
	}
	ac.CacheBatch <- &cache.Item{
		Key:   cacheKey,
		Value: marshalledNodes,
		TTL:   time.Duration(ttl) * time.Second,
	}

	ac.eraseNodesFailureMark(ctx, options, nodes, caches)

	return nodes
}

// eraseNodesFailureMark deletes the failure status on nodes on the api that were failing
// a significant amount of relays
func (ac *applicationChecks) eraseNodesFailureMark(ctx context.Context, options pocket.SyncCheckOptions, nodes []string, caches []*cache.Redis) {
	nodeFailureKey := func(blockchain, node string) string {
		return fmt.Sprintf("%s%s-%s-failure", ac.CommitHash, blockchain, node)
	}
	for _, node := range nodes {
		ac.CacheBatch <- &cache.Item{
			Key:   nodeFailureKey(options.Blockchain, node),
			Value: node,
			TTL:   1 * time.Hour,
		}
	}
}

func main() {
	lambda.Start(lambdaHandler)
}
