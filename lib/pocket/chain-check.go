package pocket

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	logger "github.com/Pocket/global-dispatcher/lib/logger"
	"github.com/Pocket/global-dispatcher/lib/metrics"
	"github.com/Pocket/global-dispatcher/lib/utils"
	"github.com/pokt-foundation/pocket-go/pkg/provider"
	"github.com/pokt-foundation/pocket-go/pkg/relayer"
	log "github.com/sirupsen/logrus"
)

type ChainChecker struct {
	Relayer         *relayer.PocketRelayer
	CommitHash      string
	MetricsRecorder *metrics.Recorder
	RequestID       string
}

type ChainCheckOptions struct {
	Session    provider.Session
	PocketAAT  provider.PocketAAT
	Blockchain string
	Data       string
	ChainID    string
	Path       string
}

type NodeChainLog struct {
	Node  *provider.Node
	Chain string
}

func (cc *ChainChecker) Check(ctx context.Context, options ChainCheckOptions) []string {
	checkedNodes := []string{}
	nodeLogs := cc.GetNodeChainLogs(ctx, &options)
	for _, node := range nodeLogs {
		publicKey := node.Node.PublicKey
		chainID := node.Chain

		if node.Chain != options.ChainID {
			logger.Log.WithFields(log.Fields{
				"sessionKey":    options.Session.Key,
				"blockhainID":   options.Blockchain,
				"requestID":     cc.RequestID,
				"serviceURL":    node.Node.ServiceURL,
				"serviceDomain": utils.GetDomainFromURL(node.Node.ServiceURL),
				"serviceNode":   node.Node.PublicKey,
			}).Warn(fmt.Sprintf("CHAIN CHECK FAILURE: %s chainiD: %s", publicKey, chainID))
			continue
		}

		logger.Log.WithFields(log.Fields{
			"sessionKey":    options.Session.Key,
			"blockhainID":   options.Blockchain,
			"requestID":     cc.RequestID,
			"serviceURL":    node.Node.ServiceURL,
			"serviceDomain": utils.GetDomainFromURL(node.Node.ServiceURL),
			"serviceNode":   node.Node.PublicKey,
		}).Info(fmt.Sprintf("CHAIN CHECK SUCCESS: %s chainiD: %s", publicKey, chainID))

		checkedNodes = append(checkedNodes, publicKey)
	}

	logger.Log.WithFields(log.Fields{
		"sessionKey":  options.Session.Key,
		"blockhainID": options.Blockchain,
		"requestID":   cc.RequestID,
	}).Info(fmt.Sprintf("CHAIN CHECK COMPLETE: %d nodes on chain", len(checkedNodes)))

	// TODO: Implement challenge

	return checkedNodes
}

func (cc *ChainChecker) GetNodeChainLogs(ctx context.Context, options *ChainCheckOptions) []*NodeChainLog {
	nodeLogsChan := make(chan *NodeChainLog, len(options.Session.Nodes))
	nodeLogs := []*NodeChainLog{}

	var wg sync.WaitGroup
	for _, node := range options.Session.Nodes {
		wg.Add(1)
		go func(n *provider.Node) {
			defer wg.Done()
			cc.GetNodeChainLog(ctx, n, nodeLogsChan, options)
		}(node)
	}
	wg.Wait()

	close(nodeLogsChan)

	for log := range nodeLogsChan {
		nodeLogs = append(nodeLogs, log)
	}

	return nodeLogs
}

func (cc *ChainChecker) GetNodeChainLog(ctx context.Context, node *provider.Node, nodeLogs chan<- *NodeChainLog, options *ChainCheckOptions) {
	start := time.Now()

	chain, err := utils.GetIntFromRelay(*cc.Relayer, relayer.RelayInput{
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
		}).Error("chain check: error relaying: ", err)

		cc.MetricsRecorder.WriteErrorMetric(ctx, &metrics.MetricData{
			Metric: &metrics.Metric{
				Timestamp:            time.Now(),
				ApplicationPublicKey: options.Session.Header.AppPublicKey,
				Blockchain:           options.Blockchain,
				NodePublicKey:        node.PublicKey,
				ElapsedTime:          time.Since(start).Seconds(),
				Bytes:                len("WRONG CHAIN"),
				Method:               "chaincheck",
				Message:              err.Error(),
			},
		})

		nodeLogs <- &NodeChainLog{
			Node:  node,
			Chain: "0",
		}
		return
	}

	nodeLogs <- &NodeChainLog{
		Node:  node,
		Chain: strconv.Itoa(int(chain)),
	}
}
