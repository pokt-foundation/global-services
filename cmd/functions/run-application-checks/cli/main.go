package main

import (
	"context"
	"encoding/json"
	"time"

	base "github.com/Pocket/global-dispatcher/cmd/functions/run-application-checks"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/common/gateway/models"
	"github.com/Pocket/global-dispatcher/lib/cache"
	"github.com/Pocket/global-dispatcher/lib/pocket"

	logger "github.com/Pocket/global-dispatcher/lib/logger"
	"github.com/Pocket/global-dispatcher/lib/utils"
	log "github.com/sirupsen/logrus"
)

var timeout = time.Duration(environment.GetInt64("TIMEOUT", 360)) * time.Second

func PerformChecks(ctx context.Context, options *base.PerformChecksOptions) {
	go func() {
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
		syncCheck(ctx, options.Ac, pocket.SyncCheckOptions{
			Session:          *options.Session,
			SyncCheckOptions: options.Blockchain.SyncCheckOptions,
			AltruistURL:      options.Blockchain.Altruist,
			Blockchain:       options.Blockchain.ID,
			PocketAAT:        *options.PocketAAT,
		}, options.Blockchain, options.CacheTTL, options.SyncCheckKey)
	}()
}

func chainCheck(ctx context.Context, ac *base.ApplicationChecks, options pocket.ChainCheckOptions, blockchain models.Blockchain, cacheTTL int, cacheKey string) []string {
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

func syncCheck(ctx context.Context, ac *base.ApplicationChecks, options pocket.SyncCheckOptions, blockchain models.Blockchain, cacheTTL int, cacheKey string) []string {
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
	err := base.RunApplicationChecks(ctx, requestID, PerformChecks)
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
