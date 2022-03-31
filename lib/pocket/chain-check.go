package pocket

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	logger "github.com/Pocket/global-dispatcher/lib/logger"
	"github.com/Pocket/global-dispatcher/lib/utils"
	"github.com/pokt-foundation/pocket-go/pkg/provider"
	"github.com/pokt-foundation/pocket-go/pkg/relayer"
	log "github.com/sirupsen/logrus"
)

type ChainChecker struct {
	Relayer    *relayer.PocketRelayer
	CommitHash string
}

type ChainCheckOptions struct {
	Session    provider.Session
	PocketAAT  provider.PocketAAT
	Blockchain string
	Data       string
	ChainID    string
	Path       string
	RequestID  string
}

type NodeChainLog struct {
	Node  *provider.Node
	Chain string
}

func (cc *ChainChecker) Check(options ChainCheckOptions) []string {
	checkedNodes := []string{}
	nodeLogs := cc.GetNodeChainLogs(&options)
	for _, node := range nodeLogs {
		publicKey := node.Node.PublicKey
		chainID := node.Chain

		if node.Chain != options.ChainID {
			logger.Log.WithFields(log.Fields{
				"sessionKey":    options.Session.Key,
				"blockhainID":   options.Blockchain,
				"requestID":     options.RequestID,
				"serviceURL":    node.Node.ServiceURL,
				"serviceDomain": utils.GetDomainFromURL(node.Node.ServiceURL),
				"serviceNode":   node.Node.PublicKey,
			}).Warn(fmt.Sprintf("CHAIN CHECK FAILURE: %s chainiD: %s", publicKey, chainID))
			continue
		}

		logger.Log.WithFields(log.Fields{
			"sessionKey":    options.Session.Key,
			"blockhainID":   options.Blockchain,
			"requestID":     options.RequestID,
			"serviceURL":    node.Node.ServiceURL,
			"serviceDomain": utils.GetDomainFromURL(node.Node.ServiceURL),
			"serviceNode":   node.Node.PublicKey,
		}).Info(fmt.Sprintf("CHAIN CHECK SUCCESS: %s chainiD: %s", publicKey, chainID))

		checkedNodes = append(checkedNodes, publicKey)
	}

	logger.Log.WithFields(log.Fields{
		"sessionKey":  options.Session.Key,
		"blockhainID": options.Blockchain,
		"requestID":   options.RequestID,
	}).Info(fmt.Sprintf("CHAIN CHECK COMPLETE: %d nodes on chain", len(checkedNodes)))

	// TODO: Implement challenge

	return checkedNodes
}

func (cc *ChainChecker) GetNodeChainLogs(options *ChainCheckOptions) []*NodeChainLog {
	nodeLogsChan := make(chan *NodeChainLog, len(options.Session.Nodes))
	nodeLogs := []*NodeChainLog{}

	var wg sync.WaitGroup
	for _, node := range options.Session.Nodes {
		wg.Add(1)
		go func(n *provider.Node) {
			defer wg.Done()
			cc.GetNodeChainLog(n, nodeLogsChan, options)
		}(node)
	}
	wg.Wait()

	close(nodeLogsChan)

	for log := range nodeLogsChan {
		nodeLogs = append(nodeLogs, log)
	}

	return nodeLogs
}

func (cc *ChainChecker) GetNodeChainLog(node *provider.Node, nodeLogs chan<- *NodeChainLog, options *ChainCheckOptions) {
	chain, err := utils.GeIntFromRelay(*cc.Relayer, relayer.RelayInput{
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
			"requestID":     options.RequestID,
			"serviceURL":    node.ServiceURL,
			"serviceDomain": utils.GetDomainFromURL(node.ServiceURL),
			"serviceNode":   node.PublicKey,
			"error":         err.Error(),
		}).Error("chain check: error relaying: ", err)

		// TODO: Send metric to error db
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
