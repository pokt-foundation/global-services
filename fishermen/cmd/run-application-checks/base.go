package base

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Pocket/global-services/shared/cache"
	"github.com/Pocket/global-services/shared/database"
	"github.com/Pocket/global-services/shared/environment"
	shared "github.com/Pocket/global-services/shared/error"
	"github.com/Pocket/global-services/shared/gateway"
	"github.com/Pocket/global-services/shared/gateway/models"
	"github.com/Pocket/global-services/shared/metrics"
	"github.com/Pocket/global-services/shared/pocket"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/pokt-foundation/pocket-go/provider"
	"github.com/pokt-foundation/pocket-go/relayer"
	"github.com/pokt-foundation/pocket-go/signer"
	"golang.org/x/sync/semaphore"

	logger "github.com/Pocket/global-services/shared/logger"
	log "github.com/sirupsen/logrus"
)

var (
	errLessThanMinimumNodes = errors.New("there are less than the minimum session nodes found")

	rpcURL                 = environment.GetString("RPC_URL", "")
	dispatchURLs           = strings.Split(environment.GetString("DISPATCH_URLS", ""), ",")
	redisConnectionStrings = strings.Split(environment.GetString("REDIS_CONNECTION_STRINGS", ""), ",")
	isRedisCluster         = environment.GetBool("IS_REDIS_CLUSTER", false)
	mongoConnectionString  = environment.GetString("MONGODB_CONNECTION_STRING", "")
	mongoDatabase          = environment.GetString("MONGODB_DATABASE", "gateway")
	cacheTTL               = environment.GetInt64("CACHE_TTL", 300)
	dispatchConcurrency    = environment.GetInt64("DISPATCH_CONCURRENCY", 30)
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
	singleApps             = strings.Split(environment.GetString("SINGLE_APPS", ""), ",")

	caches          []*cache.Redis
	metricsRecorder *metrics.Recorder
	rpcProvider     *provider.Provider
)

const emptyNodesTTL = 30

// ApplicationData saves all the info needed to run QoS checks on it
type ApplicationData struct {
	Caches          []*cache.Redis
	Provider        *provider.Provider
	Relayer         *relayer.Relayer
	MetricsRecorder *metrics.Recorder
	BlockHeight     int
	CommitHash      string
	Blockchains     map[string]*models.Blockchain
	RequestID       string
	SyncChecker     *pocket.SyncChecker
	ChainChecker    *pocket.ChainChecker
	CacheBatch      chan *cache.Item
}

// PerformChecksOptions options for the function that is going to perform the check
type PerformChecksOptions struct {
	Ac             *ApplicationData
	SyncCheckOpts  *pocket.SyncCheckOptions
	ChainCheckOpts *pocket.ChainCheckOptions
	Blockchain     models.Blockchain
	Session        *provider.Session
	PocketAAT      *provider.PocketAAT
	CacheTTL       int
	SyncCheckKey   string
	ChainCheckKey  string
	TotalApps      int
	Invalid        bool
}

// RunApplicationChecks obtains all applicationes needed to run QoS checks, performs them and
// sends successful results to be written into a cache
func RunApplicationChecks(ctx context.Context, requestID string, performChecks func(ctx context.Context, options *PerformChecksOptions)) error {
	if len(redisConnectionStrings) <= 0 {
		return shared.ErrNoCacheClientProvided
	}
	db, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return errors.New("error connecting to mongo: " + err.Error())
	}

	metricsRecorder, err = metrics.NewMetricsRecorder(ctx, &database.PostgresOptions{
		Connection:  metricsConnection,
		MinPoolSize: int(minMetricsPoolSize),
		MaxPoolSize: int(maxMetricsPoolSize),
	})
	if err != nil {
		return errors.New("error connecting to metrics db: " + err.Error())
	}

	caches, err = cache.ConnectToCacheClients(ctx, redisConnectionStrings, "", isRedisCluster)
	if err != nil {
		return errors.New("error connecting to redis: " + err.Error())
	}

	rpcProvider = provider.NewProvider(rpcURL, dispatchURLs)
	rpcProvider.UpdateRequestConfig(0, time.Duration(defaultTimeOut)*time.Second)
	signer, err := signer.NewSignerFromPrivateKey(appPrivateKey)
	if err != nil {
		return errors.New("error creating signer: " + err.Error())
	}

	relayer := relayer.NewRelayer(signer, rpcProvider)

	blockHeight, err := rpcProvider.GetBlockHeight()
	if err != nil {
		return err
	}

	ntApps, dbApps, err := gateway.GetApplicationsFromDB(ctx, db, rpcProvider, singleApps)
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

	appChecks := ApplicationData{
		Caches:          caches,
		Provider:        rpcProvider,
		Relayer:         relayer,
		MetricsRecorder: metricsRecorder,
		BlockHeight:     blockHeight,
		RequestID:       requestID,
		CacheBatch:      cacheBatch,
		SyncChecker: &pocket.SyncChecker{
			Relayer:                relayer,
			DefaultSyncAllowance:   int(defaultSyncAllowance),
			AltruistTrustThreshold: float32(altruistTrustThreshold),
			MetricsRecorder:        metricsRecorder,
			RequestID:              requestID,
		},
		ChainChecker: &pocket.ChainChecker{
			Relayer:         relayer,
			MetricsRecorder: metricsRecorder,
			RequestID:       requestID,
		},
	}

	totalApps := 0
	for _, app := range ntApps {
		for range app.Chains {
			totalApps++
		}
	}

	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(dispatchConcurrency)

	for index, app := range ntApps {
		app := app
		index := index
		for _, chain := range app.Chains {
			wg.Add(1)
			sem.Acquire(ctx, 1)
			go func(publicKey, ch string, idx int) {
				defer wg.Done()
				defer sem.Release(1)

				session, err := appChecks.getSession(ctx, publicKey, ch)

				if err != nil {
					// Such sessions cannot be dispatched so not an actual error
					if strings.Contains(err.Error(), errLessThanMinimumNodes.Error()) {
						return
					}

					logger.Log.WithFields(log.Fields{
						"appPublicKey": publicKey,
						"chain":        ch,
						"error":        err.Error(),
					}).Errorf("error dispatching: %s", err.Error())
					return
				}

				dbApp := dbApps[idx]
				pocketAAT := provider.PocketAAT{
					AppPubKey:    dbApp.GatewayAAT.ApplicationPublicKey,
					ClientPubKey: dbApp.GatewayAAT.ClientPublicKey,
					Version:      dbApp.GatewayAAT.Version,
					Signature:    dbApp.GatewayAAT.ApplicationSignature,
				}
				blockchain, ok := blockchains[ch]
				if !ok {
					blockchain = &models.Blockchain{}
				}

				performChecks(ctx, &PerformChecksOptions{
					Ac: &appChecks,
					SyncCheckOpts: &pocket.SyncCheckOptions{
						Session:          *session,
						PocketAAT:        pocketAAT,
						SyncCheckOptions: blockchain.SyncCheckOptions,
						AltruistURL:      blockchain.Altruist,
						Blockchain:       blockchain.ID,
					},
					ChainCheckOpts: &pocket.ChainCheckOptions{
						Session:    *session,
						Blockchain: blockchain.ID,
						Data:       blockchain.ChainIDCheck,
						ChainID:    blockchain.ChainID,
						Path:       blockchain.Path,
						PocketAAT:  pocketAAT,
					},
					SyncCheckKey:  appChecks.CommitHash + syncCheckKeyPrefix + session.Key,
					ChainCheckKey: appChecks.CommitHash + chainheckKeyPrefix + session.Key,
					CacheTTL:      int(cacheTTL),
					Blockchain:    *blockchain,
					Session:       session,
					PocketAAT:     &pocketAAT,
					TotalApps:     totalApps,
					Invalid:       err != nil,
				})
			}(app.PublicKey, chain, index)
		}
	}
	wg.Wait()

	close(cacheBatch)
	cacheWg.Wait()

	metricsRecorder.Close()
	return cache.CloseConnections(caches)
}

func (ac *ApplicationData) getSession(ctx context.Context, publicKey, chain string) (*provider.Session, error) {
	_, cachedSession := gateway.ShouldDispatch(ctx, ac.Caches, ac.BlockHeight,
		gateway.GetSessionCacheKey(publicKey, chain, ac.CommitHash), int(maxClientsCacheCheck))

	if cachedSession != nil {
		return cachedSession, nil
	}

	dispatch, err := ac.Provider.Dispatch(publicKey, chain, nil)
	if err != nil {
		return nil, err
	}

	if dispatch.Session.Key == "" {
		return nil, errors.New("empty session")
	}

	return dispatch.Session, nil
}

// EraseNodesFailureMark deletes the failure status on nodes on the api that were failing
// a significant amount of relays
func EraseNodesFailureMark(nodes []string, blockchain, commitHash string, cacheBatch chan *cache.Item) {
	nodeFailureKey := func(blockchain, commitHash, node string) string {
		return fmt.Sprintf("%s{%s}-%s-failure", commitHash, blockchain, node)
	}
	for _, node := range nodes {
		cacheBatch <- &cache.Item{
			Key:   nodeFailureKey(blockchain, commitHash, node),
			Value: false,
			TTL:   1 * time.Hour,
		}
	}
}

// CacheNodes inserts a into the cache batch channel
func CacheNodes(nodes []string, batch chan *cache.Item, key string, ttl int) error {
	marshalledNodes, err := json.Marshal(nodes)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		ttl = emptyNodesTTL
	}
	batch <- &cache.Item{
		Key:   key,
		Value: marshalledNodes,
		TTL:   time.Duration(ttl) * time.Second,
	}

	return nil
}
