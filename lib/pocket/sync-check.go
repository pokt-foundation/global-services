package pocket

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Pocket/global-dispatcher/common/gateway/models"
	logger "github.com/Pocket/global-dispatcher/lib/logger"
	"github.com/Pocket/global-dispatcher/lib/metrics"
	"github.com/Pocket/global-dispatcher/lib/utils"
	"github.com/pokt-foundation/pocket-go/pkg/provider"
	"github.com/pokt-foundation/pocket-go/pkg/relayer"
	log "github.com/sirupsen/logrus"

	_http "github.com/Pocket/global-dispatcher/lib/http"
)

var httpClient = _http.NewClient()

type SyncChecker struct {
	Relayer                *relayer.PocketRelayer
	DefaultSyncAllowance   int
	AltruistTrustThreshold float32
	MetricsRecorder        *metrics.Recorder
	RequestID              string
}

type SyncCheckOptions struct {
	Session          provider.Session
	PocketAAT        provider.PocketAAT
	SyncCheckOptions models.SyncCheckOptions
	Path             string
	AltruistURL      string
	Blockchain       string
}

type NodeSyncLog struct {
	Node        *provider.Node
	BlockHeight int64
}

func (sc *SyncChecker) Check(ctx context.Context, options SyncCheckOptions) []string {
	if options.SyncCheckOptions.Allowance == 0 {
		options.SyncCheckOptions.Allowance = sc.DefaultSyncAllowance
	}
	allowance := int64(options.SyncCheckOptions.Allowance)

	checkedNodes := []string{}
	nodeLogs := sc.GetNodeSyncLogs(ctx, &options)
	sort.Slice(nodeLogs, func(i, j int) bool {
		return nodeLogs[i].BlockHeight > nodeLogs[j].BlockHeight
	})

	altruistBlockHeight, highestBlockHeight, isAltruistTrustworthy := sc.GetAltruistDataAndHighestBlockHeight(nodeLogs, &options)

	maxAllowedBlockHeight := int64(0)
	if isAltruistTrustworthy {
		maxAllowedBlockHeight = altruistBlockHeight + allowance
	} else {
		maxAllowedBlockHeight = highestBlockHeight + allowance
	}

	for _, node := range nodeLogs {
		publicKey := node.Node.PublicKey
		blockHeight := node.BlockHeight
		allowanceBlockHeight := blockHeight + allowance

		isValidNode := blockHeight <= maxAllowedBlockHeight &&
			allowanceBlockHeight >= highestBlockHeight &&
			allowanceBlockHeight >= altruistBlockHeight

		if blockHeight <= 0 {
			continue
		}

		if !isValidNode {
			logger.Log.WithFields(log.Fields{
				"sessionKey":    options.Session.Key,
				"blockhainID":   options.Blockchain,
				"requestID":     sc.RequestID,
				"serviceURL":    node.Node.ServiceURL,
				"serviceDomain": utils.GetDomainFromURL(node.Node.ServiceURL),
				"serviceNode":   node.Node.PublicKey,
			}).Warn(fmt.Sprintf("SYNC CHECK BEHIND: %s height: %d", publicKey, blockHeight))
			continue
		}

		logger.Log.WithFields(log.Fields{
			"sessionKey":    options.Session.Key,
			"blockhainID":   options.Blockchain,
			"requestID":     sc.RequestID,
			"serviceURL":    node.Node.ServiceURL,
			"serviceDomain": utils.GetDomainFromURL(node.Node.ServiceURL),
			"serviceNode":   node.Node.PublicKey,
			"allowance":     options.SyncCheckOptions.Allowance,
		}).Info(fmt.Sprintf("SYNC CHECK IN-SYNC: %s height: %d", publicKey, blockHeight))

		checkedNodes = append(checkedNodes, publicKey)
	}

	logger.Log.WithFields(log.Fields{
		"sessionKey":  options.Session.Key,
		"blockhainID": options.Blockchain,
		"requestID":   sc.RequestID,
	}).Info(fmt.Sprintf("SYNC CHECK COMPLETE: %d nodes in sync", len(checkedNodes)))

	// TODO: Implement challenge

	return checkedNodes
}

func (sc *SyncChecker) GetNodeSyncLogs(ctx context.Context, options *SyncCheckOptions) []*NodeSyncLog {
	nodeLogsChan := make(chan *NodeSyncLog, len(options.Session.Nodes))
	nodeLogs := []*NodeSyncLog{}

	var wg sync.WaitGroup
	for _, node := range options.Session.Nodes {
		wg.Add(1)
		go func(n *provider.Node) {
			defer wg.Done()
			sc.GetNodeSyncLog(ctx, n, nodeLogsChan, options)
		}(node)
	}
	wg.Wait()

	close(nodeLogsChan)

	for log := range nodeLogsChan {
		nodeLogs = append(nodeLogs, log)
	}

	return nodeLogs
}

func (sc *SyncChecker) GetNodeSyncLog(ctx context.Context, node *provider.Node, nodeLogs chan<- *NodeSyncLog, options *SyncCheckOptions) {
	start := time.Now()

	blockHeight, err := utils.GetIntFromRelay(*sc.Relayer, relayer.RelayInput{
		Blockchain: options.Blockchain,
		Data:       strings.Replace(options.SyncCheckOptions.Body, `\`, "", -1),
		Method:     http.MethodPost,
		PocketAAT:  &options.PocketAAT,
		Session:    &options.Session,
		Node:       node,
		Path:       options.Path,
	}, nil, options.SyncCheckOptions.ResultKey)

	if err != nil {
		logger.Log.WithFields(log.Fields{
			"sessionKey":    options.Session.Key,
			"blockhainID":   options.Blockchain,
			"requestID":     sc.RequestID,
			"serviceURL":    node.ServiceURL,
			"serviceDomain": utils.GetDomainFromURL(node.ServiceURL),
			"error":         err.Error(),
		}).Error("sync check: error obtaining block height: " + err.Error())

		go sc.MetricsRecorder.WriteErrorMetric(ctx, &metrics.MetricData{
			Metric: &metrics.Metric{
				Timestamp:            time.Now(),
				ApplicationPublicKey: options.Session.Header.AppPublicKey,
				Blockchain:           options.Blockchain,
				NodePublicKey:        node.PublicKey,
				ElapsedTime:          time.Since(start).Seconds(),
				Bytes:                len(err.Error()),
				Method:               "synccheck",
				Message:              err.Error(),
			},
		})

		nodeLogs <- &NodeSyncLog{
			Node:        node,
			BlockHeight: 0,
		}
		return
	}

	nodeLogs <- &NodeSyncLog{
		Node:        node,
		BlockHeight: blockHeight,
	}
}

func (sc *SyncChecker) GetAltruistDataAndHighestBlockHeight(nodeLogs []*NodeSyncLog, options *SyncCheckOptions) (altruistBlockHeight, highestBlockHeight int64, isAltruistTrustworthy bool) {
	validNodes, highestBlockHeight := sc.getValidNodesCountAndHighestNode(nodeLogs, options)

	altruistBlockHeight, nodesAheadOfAltruist := sc.getValidatedAltruist(nodeLogs, options)

	// Prevents division by 0 in case all nodes fail
	divisionValidNodes := validNodes
	if divisionValidNodes <= 0 {
		divisionValidNodes = 1
	}

	isAltruistTrustworthy = float32(nodesAheadOfAltruist/divisionValidNodes) < sc.AltruistTrustThreshold

	if !isAltruistTrustworthy {
		logger.Log.WithFields(log.Fields{
			"sessionKey":  options.Session.Key,
			"blockhainID": options.Blockchain,
			"requestID":   sc.RequestID,
		},
		).Error(fmt.Sprintf("sync check: altruist failure: %d out of %d sync nodes are ahead of altruist", nodesAheadOfAltruist, validNodes))

		// Altruist can't be trusted
		altruistBlockHeight = highestBlockHeight
	}

	// Nodes from other chains can present values too ahead from the blockchain currently checked
	isHighestBlockHeightTooFar := highestBlockHeight > altruistBlockHeight+int64(options.SyncCheckOptions.Allowance)
	if isAltruistTrustworthy && isHighestBlockHeightTooFar {
		highestBlockHeight = altruistBlockHeight
	}

	return
}

func (sc *SyncChecker) getValidNodesCountAndHighestNode(nodeLogs []*NodeSyncLog, options *SyncCheckOptions) (validNodes int, highestBlockHeight int64) {
	validNodes = func() int {
		validNodesCount := 0
		for _, node := range nodeLogs {
			if node.BlockHeight > 0 {
				validNodesCount++
			}
		}
		return validNodesCount
	}()
	// This should never happen
	if validNodes <= 2 {
		logger.Log.WithFields(log.Fields{
			"sessionKey":  options.Session.Key,
			"blockhainID": options.Blockchain,
			"requestID":   sc.RequestID,
		}).Error("sync check error: fewer than 3 nodes returned sync")
	}

	highestBlockHeight = nodeLogs[0].BlockHeight
	if highestBlockHeight <= 0 {
		logger.Log.WithFields(log.Fields{
			"sessionKey":  options.Session.Key,
			"blockhainID": options.Blockchain,
			"requestID":   sc.RequestID,
		}).Error("sync check: top synced node result is invalid")
	}

	return
}

// getValidatedAltruist obtains and validates altruist block height and also returns,
// how many nodes are ahead of it
func (sc *SyncChecker) getValidatedAltruist(nodeLogs []*NodeSyncLog, options *SyncCheckOptions) (int64, int) {
	altruistBlockHeight, err := getAltruistBlockHeight(options.SyncCheckOptions, options.AltruistURL)
	if altruistBlockHeight == 0 || err != nil {
		logger.Log.WithFields(log.Fields{
			"sessionKey":  options.Session.Key,
			"blockhainID": options.Blockchain,
			"requestID":   sc.RequestID,
			"serviceNode": "ALTRUIST",
		}).Error("sync check: altruist failure: ", err)
	}
	logger.Log.WithFields(log.Fields{
		"sessionKey":  options.Session.Key,
		"blockhainID": options.Blockchain,
		"requestID":   sc.RequestID,
		"relayType":   "FALLBACK",
		"blockHeight": altruistBlockHeight,
	}).Info("sync check: altruist check: ", altruistBlockHeight)

	nodesAheadOfAltruist := func() int {
		nodesAheadOfAltruistCount := 0
		for _, node := range nodeLogs {
			if node.BlockHeight > altruistBlockHeight {
				nodesAheadOfAltruistCount++
			}
		}
		return nodesAheadOfAltruistCount
	}()

	return altruistBlockHeight, nodesAheadOfAltruist
}

func getAltruistBlockHeight(options models.SyncCheckOptions, altruistURL string) (int64, error) {
	regex := regexp.MustCompile(`/[\w]*:\/\/[^\/]*@/g`)
	regex.ReplaceAllString(altruistURL, "")

	req, err := http.NewRequest(http.MethodPost, altruistURL,
		bytes.NewBuffer([]byte(strings.Replace(options.Body, `\`, "", -1))))
	defer utils.CloseOrLog(req.Response)
	if err != nil {
		return 0, errors.New("error making altruist request: " + err.Error())
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := httpClient.Do(req)
	defer utils.CloseOrLog(req.Response)
	if err != nil {
		return 0, errors.New("error performing altruist request: " + err.Error())
	}

	return utils.ParseIntegerFromPayload(res.Body, options.ResultKey)
}
