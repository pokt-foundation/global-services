package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	base "github.com/Pocket/global-services/fishermen/cmd/run-application-checks"
	"github.com/Pocket/global-services/shared/cache"
	"github.com/Pocket/global-services/shared/environment"
	"github.com/Pocket/global-services/shared/gateway/models"
	"github.com/Pocket/global-services/shared/pocket"
	"github.com/Pocket/global-services/shared/utils"

	logger "github.com/Pocket/global-services/shared/logger"
	log "github.com/sirupsen/logrus"
)

var timeout = time.Duration(environment.GetInt64("TIMEOUT", 360)) * time.Second

func performChecks(ctx context.Context, options *base.PerformChecksOptions) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		chainCheck(ctx, options.Ac, pocket.ChainCheckOptions{
			Session:    *options.Session,
			Blockchain: options.Blockchain.ID,
			Data:       options.Blockchain.ChainIDCheck,
			ChainID:    options.Blockchain.ChainID,
			Path:       options.Blockchain.Path,
			PocketAAT:  *options.PocketAAT,
		}, options.Blockchain, options.CacheTTL, options.ChainCheckKey)
	}()

	go func() {
		defer wg.Done()
		syncCheck(ctx, options.Ac, pocket.SyncCheckOptions{
			Session:          *options.Session,
			SyncCheckOptions: options.Blockchain.SyncCheckOptions,
			AltruistURL:      options.Blockchain.Altruist,
			Blockchain:       options.Blockchain.ID,
			PocketAAT:        *options.PocketAAT,
		}, options.Blockchain, options.CacheTTL, options.SyncCheckKey)
	}()
	wg.Wait()
}

func chainCheck(ctx context.Context, ac *base.ApplicationData, options pocket.ChainCheckOptions, blockchain models.Blockchain, cacheTTL int, cacheKey string) []string {
	if blockchain.ChainIDCheck == "" {
		return []string{}
	}

	nodes := ac.ChainChecker.Check(ctx, options)
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

func syncCheck(ctx context.Context, ac *base.ApplicationData, options pocket.SyncCheckOptions, blockchain models.Blockchain, cacheTTL int, cacheKey string) []string {
	if blockchain.SyncCheckOptions.Body == "" && blockchain.SyncCheckOptions.Path == "" {
		return []string{}
	}

	nodes := ac.SyncChecker.Check(ctx, options)
	if err := base.CacheNodes(nodes, ac.CacheBatch, cacheKey, cacheTTL); err != nil {
		logger.Log.WithFields(log.Fields{
			"error":        err.Error(),
			"requestID":    ac.RequestID,
			"blockchainID": blockchain.ID,
			"sessionKey":   options.Session.Key,
		}).Error("syncc check: error caching sync check nodes: " + err.Error())
	}
	base.EraseNodesFailureMark(nodes, blockchain.ID, ac.CommitHash, ac.CacheBatch)

	return nodes
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	requestID, _ := utils.RandomHex(32)
	err := base.RunApplicationChecks(ctx, requestID, performChecks)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": requestID,
			"error":     err.Error(),
		}).Error("ERROR RUNNING APPLICATION CHECKS: " + err.Error())
	}
	logger.Log.WithFields(log.Fields{
		"requestID": requestID,
	}).Info("RUN APPLICATION CHECKS RESULT")
}
