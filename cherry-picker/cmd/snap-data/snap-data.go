package snapdata

import (
	"context"
	"encoding/json"
	"strings"

	cpicker "github.com/Pocket/global-services/cherry-picker"
	db "github.com/Pocket/global-services/cherry-picker/database"
	"github.com/Pocket/global-services/shared/cache"
	"github.com/Pocket/global-services/shared/database"
	"github.com/Pocket/global-services/shared/environment"
	shared "github.com/Pocket/global-services/shared/error"
	"golang.org/x/exp/slices"
)

var (
	cherryPickerConnections = strings.Split(environment.GetString("CHERRY_PICKER_CONNECTIONS", ""), ",")
	redisConnectionStrings  = environment.GetString("REDIS_REGION_CONNECTION_STRINGS", "")
	isRedisCluster          = environment.GetBool("IS_REDIS_CLUSTER", false)
	concurrency             = environment.GetInt64("CONCURRENCY", 1)
	successKey              = environment.GetString("SUCCESS_KEY", "success-hits")
	failuresKey             = environment.GetString("SUCCESS_KEY", "failure-hits")
	failureKey              = environment.GetString("SUCCESS_KEY", "failure")
	sessionTableName        = environment.GetString("SESSION_TABLE_NAME", "cherry_picker_session")
	sessionRegionTableName  = environment.GetString("SESSION_REGION_TABLE_NAME", "cherry_picker_session_region")
	minPoolSize             = environment.GetInt64("MIN_POOL_SIZE", 100)
	maxPoolSize             = environment.GetInt64("MAX_POOL_SIZE", 200)
)

type ApplicationData struct {
	ServiceLog             cpicker.ServiceLog
	Address                string
	Successes              int
	Failures               int
	Failure                bool
	PublicKey              string
	Chain                  string
	MedianSuccessLatency   float32
	WeightedSuccessLatency float32
}

type Region struct {
	Cache   *cache.Redis
	Name    string
	AppData map[string]*ApplicationData
}

type SessionKeys struct {
	PublicKey  string
	Chain      string
	SessionKey string
}

type SnapCherryPicker struct {
	Regions   map[string]*Region
	Caches    []*cache.Redis
	Stores    []cpicker.CherryPickerStore
	RequestID string
}

func (sn *SnapCherryPicker) Init(ctx context.Context) error {
	if err := sn.initRegionCaches(ctx); err != nil {
		return err
	}

	for _, connString := range cherryPickerConnections {
		connection, err := db.NewCherryPickerPostgresFromConnectionString(ctx, &database.PostgresOptions{
			Connection:  connString,
			MinPoolSize: int(minPoolSize),
			MaxPoolSize: int(maxPoolSize),
		}, sessionTableName, sessionRegionTableName)
		if err != nil {
			return err
		}
		sn.Stores = append(sn.Stores, connection)
	}

	return nil
}

func (sn *SnapCherryPicker) initRegionCaches(ctx context.Context) error {
	var cacheRegionConns map[string]string

	if err := json.Unmarshal([]byte(redisConnectionStrings), &cacheRegionConns); err != nil {
		return err
	}

	if len(cacheRegionConns) == 0 {
		return shared.ErrNoCacheClientProvided
	}

	conns := []string{}
	for _, connStr := range cacheRegionConns {
		conns = append(conns, connStr)
	}

	caches, err := cache.ConnectoCacheClients(ctx, conns, "", isRedisCluster)
	if err != nil {
		return err
	}

	for region, connStr := range cacheRegionConns {
		idx := slices.IndexFunc(caches, func(ch *cache.Redis) bool {
			return ch.Addrs()[0] == connStr
		})

		ch := caches[idx]
		ch.Name = region
		sn.Regions[region] = &Region{
			Cache:   ch,
			Name:    region,
			AppData: make(map[string]*ApplicationData),
		}
		sn.Caches = append(sn.Caches, ch)
	}

	return nil
}

// SnapCherryPickerData obtains service node data from all cache instances
// and saves to the stores available
func (sn *SnapCherryPicker) SnapCherryPickerData(ctx context.Context) error {
	if err := sn.getAppsRegionsData(ctx); err != nil {
		return err
	}
	sn.saveToStore(ctx)
	return nil
}
