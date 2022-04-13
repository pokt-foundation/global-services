package base

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/common/gateway"
	"github.com/Pocket/global-dispatcher/common/gateway/models"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/database"
	"github.com/Pocket/global-dispatcher/lib/metrics"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/Pocket/global-dispatcher/lib/utils"
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
	mongoDatabase          = environment.GetString("MONGODB_DATABASE", "gateway")
	cacheTTL               = environment.GetInt64("CACHE_TTL", 300)
	dispatchConcurrency    = environment.GetInt64("DISPATCH_CONCURRENCY", 30)
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

const EMPTY_NODES_TTL = 30

type ApplicationChecks struct {
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

type PerformChecksOptions struct {
	Wg             *sync.WaitGroup
	Sem            *semaphore.Weighted
	Ac             *ApplicationChecks
	SyncCheckOpts  *pocket.SyncCheckOptions
	ChainCheckOpts *pocket.ChainCheckOptions
	Blockchain     models.Blockchain
	Session        *provider.Session
	PocketAAT      *provider.PocketAAT
	CacheTTL       int
	SyncCheckKey   string
	ChainCheckKey  string
}

func RunApplicationChecks(ctx context.Context, requestID string, performChecks func(ctx context.Context, options *PerformChecksOptions)) error {
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

	appChecks := ApplicationChecks{
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

	var wg sync.WaitGroup
	var sem = semaphore.NewWeighted(dispatchConcurrency)
	for index, app := range ntApps {
		for _, chain := range app.Chains {
			wg.Add(1)
			go func(publicKey, ch string, idx int) {
				defer wg.Done()

				session, err := appChecks.getSession(ctx, publicKey, ch)
				if err != nil {
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
				blockchain := *blockchains[ch]

				go performChecks(ctx, &PerformChecksOptions{
					Wg:  &wg,
					Sem: sem,
					Ac:  &appChecks,
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
					Blockchain:    blockchain,
					Session:       session,
					PocketAAT:     &pocketAAT,
				})
			}(app.PublicKey, chain, index)
		}
	}
	wg.Wait()

	close(cacheBatch)
	cacheWg.Wait()

	metricsRecorder.Conn.Close()
	return cache.CloseConnections(caches)
}

func (ac *ApplicationChecks) getSession(ctx context.Context, publicKey, chain string) (*provider.Session, error) {
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

// EraseNodesFailureMark deletes the failure status on nodes on the api that were failing
// a significant amount of relays
func EraseNodesFailureMark(nodes []string, blockchain, commitHash string, cacheBatch chan *cache.Item) {
	nodeFailureKey := func(blockchain, commitHash, node string) string {
		return fmt.Sprintf("%s%s-%s-failure", commitHash, blockchain, node)
	}
	for _, node := range nodes {
		cacheBatch <- &cache.Item{
			Key:   nodeFailureKey(blockchain, commitHash, node),
			Value: node,
			TTL:   1 * time.Hour,
		}
	}
}

func CacheNodes(nodes []string, batch chan *cache.Item, key string, ttl int) error {
	marshalledNodes, err := json.Marshal(nodes)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		ttl = EMPTY_NODES_TTL
	}
	batch <- &cache.Item{
		Key:   key,
		Value: marshalledNodes,
		TTL:   time.Duration(ttl) * time.Second,
	}

	return nil
}
