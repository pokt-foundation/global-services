package pocket

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	logger "github.com/Pocket/global-services/shared/logger"
	"github.com/Pocket/global-services/shared/metrics"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/pokt-foundation/pocket-go/provider"
	"github.com/pokt-foundation/pocket-go/relayer"
	log "github.com/sirupsen/logrus"
)

// ChainChecker is the struct to perform chain checks on app sessions
type ChainChecker struct {
	Relayer         *relayer.Relayer
	MetricsRecorder *metrics.Recorder
	RequestID       string
}

// ChainCheckOptions is the struct of the data needed to perform a chain check
type ChainCheckOptions struct {
	Session    provider.Session
	PocketAAT  provider.PocketAAT
	Blockchain string
	Data       string
	ChainID    string
	Path       string
}

type nodeChainLog struct {
	Node  *provider.Node
	Chain int64
}

// Check Performs a chain check of all the nodes of the given session
func (cc *ChainChecker) Check(ctx context.Context, options ChainCheckOptions) []string {
	checkedNodes := []string{}

	chainID, err := strconv.Atoi(options.ChainID)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"sessionKey":  options.Session.Key,
			"blockhainID": options.Blockchain,
			"requestID":   cc.RequestID,
		}).Errorf("chain check: error parsing blockchain's chainID: %s", err.Error())

		return checkedNodes
	}

	nodeLogs := cc.getNodeChainLogs(ctx, &options)
	for _, node := range nodeLogs {
		publicKey := node.Node.PublicKey
		nodeChainID := node.Chain

		if nodeChainID != int64(chainID) {
			logger.Log.WithFields(log.Fields{
				"sessionKey":            options.Session.Key,
				"blockhainID":           options.Blockchain,
				"requestID":             cc.RequestID,
				"serviceURL":            node.Node.ServiceURL,
				"serviceDomain":         utils.GetDomainFromURL(node.Node.ServiceURL),
				"serviceNode":           node.Node.PublicKey,
				"appplicationPublicKey": options.Session.Header.AppPublicKey,
			}).Warn(fmt.Sprintf("CHAIN CHECK FAILURE: %s chainiD: %d", publicKey, nodeChainID))
			continue
		}

		logger.Log.WithFields(log.Fields{
			"sessionKey":            options.Session.Key,
			"blockhainID":           options.Blockchain,
			"requestID":             cc.RequestID,
			"serviceURL":            node.Node.ServiceURL,
			"serviceDomain":         utils.GetDomainFromURL(node.Node.ServiceURL),
			"serviceNode":           node.Node.PublicKey,
			"appplicationPublicKey": options.Session.Header.AppPublicKey,
		}).Info(fmt.Sprintf("CHAIN CHECK SUCCESS: %s chainiD: %d", publicKey, nodeChainID))

		checkedNodes = append(checkedNodes, publicKey)
	}

	logger.Log.WithFields(log.Fields{
		"sessionKey":            options.Session.Key,
		"blockhainID":           options.Blockchain,
		"requestID":             cc.RequestID,
		"appplicationPublicKey": options.Session.Header.AppPublicKey,
	}).Info(fmt.Sprintf("CHAIN CHECK COMPLETE: %d nodes on chain", len(checkedNodes)))

	// TODO: Implement challenge

	return checkedNodes
}

func (cc *ChainChecker) getNodeChainLogs(ctx context.Context, options *ChainCheckOptions) []*nodeChainLog {
	nodeLogsChan := make(chan *nodeChainLog, len(options.Session.Nodes))
	nodeLogs := []*nodeChainLog{}

	var wg sync.WaitGroup
	for _, node := range options.Session.Nodes {
		wg.Add(1)
		go func(n *provider.Node) {
			defer wg.Done()
			cc.getNodeChainLog(ctx, n, nodeLogsChan, options)
		}(node)
	}
	wg.Wait()

	close(nodeLogsChan)

	for log := range nodeLogsChan {
		nodeLogs = append(nodeLogs, log)
	}

	return nodeLogs
}

func (cc *ChainChecker) getNodeChainLog(ctx context.Context, node *provider.Node, nodeLogs chan<- *nodeChainLog, options *ChainCheckOptions) {
	start := time.Now()

	chain, err := utils.GetIntFromRelay(*cc.Relayer, relayer.Input{
		Blockchain: options.Blockchain,
		Data:       strings.Replace(options.Data, `\`, "", -1),
		Method:     http.MethodPost,
		PocketAAT:  &options.PocketAAT,
		Session:    &options.Session,
		Node:       node,
		Path:       options.Path,
	}, "result")
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"sessionKey":    options.Session.Key,
			"blockhainID":   options.Blockchain,
			"requestID":     cc.RequestID,
			"serviceURL":    node.ServiceURL,
			"serviceDomain": utils.GetDomainFromURL(node.ServiceURL),
			"serviceNode":   node.PublicKey,
			"error":         err.Error(),
		}).Error("chain check: error obtaining chain ID: ", err)

		cc.MetricsRecorder.WriteErrorMetric(ctx, &metrics.Metric{
			Timestamp:            time.Now(),
			ApplicationPublicKey: options.Session.Header.AppPublicKey,
			Blockchain:           options.Blockchain,
			NodePublicKey:        node.PublicKey,
			ElapsedTime:          time.Since(start).Seconds(),
			Bytes:                len("WRONG CHAIN"),
			Method:               "chaincheck",
			Message:              err.Error(),
			RequestID:            cc.RequestID,
		})

		nodeLogs <- &nodeChainLog{
			Node:  node,
			Chain: 0,
		}
		return
	}

	nodeLogs <- &nodeChainLog{
		Node:  node,
		Chain: chain,
	}
}
