package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/lib/cache"
	httpClient "github.com/Pocket/global-dispatcher/lib/http"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/Pocket/global-dispatcher/lib/utils"
)

var gatewayURL = environment.GetString("GATEWAY_PRODUCTION_URL", "")
var sessionKeyPrefix = environment.GetString("SESSION_KEY_PREFIX", "")

const versionPath = "/version"

// GetGatewayCommitHash returns the current gateway commit hash
// TODO: Consumers have badly configured the commithash prefix and right now they
// don't use any kind of prefix on their cache keys.
func GetGatewayCommitHash() (string, error) {
	httpClient := *httpClient.NewClient()
	res, err := httpClient.Get(gatewayURL+versionPath, nil)
	if err != nil {
		return "", err
	}

	var commitHash struct {
		Commit string `json:"commit"`
	}

	return commitHash.Commit, json.NewDecoder(res.Body).Decode(&commitHash)
}

// GetSessionCacheKey returns the session cache key
func GetSessionCacheKey(publicKey, chain, commitHash string) string {
	return fmt.Sprintf("%s%s-%s-%s", commitHash, sessionKeyPrefix, publicKey, chain)
}

// ShouldDispatch checks N random cache clients and checks whether the session
// is available and up to date with the current block, fails if any of the
// clients fails the check and returns the session if found by any cache
func ShouldDispatch(ctx context.Context, caches []*cache.Redis, blockHeight int, key string, maxClients int) (bool, *pocket.SessionCamelCase) {
	clientsToCheck := utils.Min(len(caches), maxClients)
	clients := utils.Shuffle(caches)[0:clientsToCheck]
	var globalCachedSession *pocket.SessionCamelCase

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
			globalCachedSession = &cachedSession
		}(client)
	}
	wg.Wait()

	return cachedClients != uint32(clientsToCheck), globalCachedSession
}
