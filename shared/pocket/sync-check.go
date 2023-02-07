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

	logger "github.com/Pocket/global-services/shared/logger"
	"github.com/Pocket/global-services/shared/metrics"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/pokt-foundation/pocket-go/provider"
	"github.com/pokt-foundation/pocket-go/relayer"
	"github.com/pokt-foundation/portal-db/types"
	log "github.com/sirupsen/logrus"

	_http "github.com/Pocket/global-services/shared/http"
)

var httpClient = _http.NewClient()

// SyncChecker is the struct to perform sync checks on app sessions
type SyncChecker struct {
	Relayer                *relayer.Relayer
	DefaultSyncAllowance   int
	AltruistTrustThreshold float32
	MetricsRecorder        *metrics.Recorder
	RequestID              string
}

// SyncCheckOptions is the struct of the data needed to perform a sync check
type SyncCheckOptions struct {
	Session          provider.Session
	PocketAAT        provider.PocketAAT
	SyncCheckOptions types.SyncCheckOptions
	AltruistURL      string
	Blockchain       string
}

type nodeSyncLog struct {
	Node        *provider.Node
	BlockHeight int64
}

// Check performs a sync check of all the nodes of a given session
func (sc *SyncChecker) Check(ctx context.Context, options SyncCheckOptions) []string {
	if options.SyncCheckOptions.Allowance == 0 {
		options.SyncCheckOptions.Allowance = sc.DefaultSyncAllowance
	}
	allowance := int64(options.SyncCheckOptions.Allowance)

	checkedNodes := []string{}
	nodeLogs := sc.getNodeSyncLogs(ctx, &options)
	sort.Slice(nodeLogs, func(i, j int) bool {
		return nodeLogs[i].BlockHeight > nodeLogs[j].BlockHeight
	})

	altruistBlockHeight, highestBlockHeight, isAltruistTrustworthy := sc.getAltruistDataAndHighestBlockHeight(nodeLogs, &options)

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
				"sessionKey":            options.Session.Key,
				"blockchainID":          options.Blockchain,
				"requestID":             sc.RequestID,
				"serviceURL":            node.Node.ServiceURL,
				"serviceDomain":         utils.GetDomainFromURL(node.Node.ServiceURL),
				"serviceNode":           node.Node.PublicKey,
				"appplicationPublicKey": options.Session.Header.AppPublicKey,
			}).Warn(fmt.Sprintf("SYNC CHECK BEHIND: %s height: %d", publicKey, blockHeight))
			continue
		}

		logger.Log.WithFields(log.Fields{
			"sessionKey":            options.Session.Key,
			"blockchainID":          options.Blockchain,
			"requestID":             sc.RequestID,
			"serviceURL":            node.Node.ServiceURL,
			"serviceDomain":         utils.GetDomainFromURL(node.Node.ServiceURL),
			"serviceNode":           node.Node.PublicKey,
			"allowance":             options.SyncCheckOptions.Allowance,
			"appplicationPublicKey": options.Session.Header.AppPublicKey,
		}).Info(fmt.Sprintf("SYNC CHECK IN-SYNC: %s height: %d", publicKey, blockHeight))

		checkedNodes = append(checkedNodes, publicKey)
	}

	logger.Log.WithFields(log.Fields{
		"sessionKey":            options.Session.Key,
		"blockchainID":          options.Blockchain,
		"requestID":             sc.RequestID,
		"appplicationPublicKey": options.Session.Header.AppPublicKey,
	}).Info(fmt.Sprintf("SYNC CHECK COMPLETE: %d nodes in sync", len(checkedNodes)))

	// TODO: Implement challenge

	return checkedNodes
}

func (sc *SyncChecker) getNodeSyncLogs(ctx context.Context, options *SyncCheckOptions) []*nodeSyncLog {
	nodeLogsChan := make(chan *nodeSyncLog, len(options.Session.Nodes))
	nodeLogs := []*nodeSyncLog{}

	var wg sync.WaitGroup
	for _, node := range options.Session.Nodes {
		wg.Add(1)
		go func(n *provider.Node) {
			defer wg.Done()
			sc.getNodeSyncLog(ctx, n, nodeLogsChan, options)
		}(node)
	}
	wg.Wait()

	close(nodeLogsChan)

	for log := range nodeLogsChan {
		nodeLogs = append(nodeLogs, log)
	}

	return nodeLogs
}

func (sc *SyncChecker) getNodeSyncLog(ctx context.Context, node *provider.Node, nodeLogs chan<- *nodeSyncLog, options *SyncCheckOptions) {
	start := time.Now()

	blockHeight, err := utils.GetIntFromRelay(*sc.Relayer, relayer.Input{
		Blockchain: options.Blockchain,
		Data:       strings.Replace(options.SyncCheckOptions.Body, `\`, "", -1),
		Method:     http.MethodPost,
		PocketAAT:  &options.PocketAAT,
		Session:    &options.Session,
		Node:       node,
		Path:       options.SyncCheckOptions.Path,
	}, options.SyncCheckOptions.ResultKey)

	if err != nil {
		logger.Log.WithFields(log.Fields{
			"sessionKey":    options.Session.Key,
			"blockchainID":  options.Blockchain,
			"requestID":     sc.RequestID,
			"serviceURL":    node.ServiceURL,
			"serviceDomain": utils.GetDomainFromURL(node.ServiceURL),
			"error":         err.Error(),
		}).Error("sync check: error obtaining block height: " + err.Error())

		sc.MetricsRecorder.WriteErrorMetric(ctx, &metrics.Metric{
			Timestamp:            time.Now(),
			ApplicationPublicKey: options.Session.Header.AppPublicKey,
			Blockchain:           options.Blockchain,
			NodePublicKey:        node.PublicKey,
			ElapsedTime:          time.Since(start).Seconds(),
			Bytes:                len(err.Error()),
			Method:               "synccheck",
			Message:              err.Error(),
			RequestID:            sc.RequestID,
		})

		nodeLogs <- &nodeSyncLog{
			Node:        node,
			BlockHeight: 0,
		}
		return
	}

	nodeLogs <- &nodeSyncLog{
		Node:        node,
		BlockHeight: blockHeight,
	}
}

func (sc *SyncChecker) getAltruistDataAndHighestBlockHeight(nodeLogs []*nodeSyncLog, options *SyncCheckOptions) (altruistBlockHeight, highestBlockHeight int64, isAltruistTrustworthy bool) {
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
			"sessionKey":   options.Session.Key,
			"blockchainID": options.Blockchain,
			"requestID":    sc.RequestID,
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

	return altruistBlockHeight, highestBlockHeight, isAltruistTrustworthy
}

func (sc *SyncChecker) getValidNodesCountAndHighestNode(nodeLogs []*nodeSyncLog, options *SyncCheckOptions) (validNodes int, highestBlockHeight int64) {
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
	errMsg := "sync check error: fewer than 3 nodes returned sync"
	if validNodes <= 2 {
		logger.Log.WithFields(log.Fields{
			"sessionKey":   options.Session.Key,
			"blockchainID": options.Blockchain,
			"requestID":    sc.RequestID,
			"error":        errMsg,
		}).Error(errMsg)
	}

	errMsg = "sync check: top synced node result is invalid"
	highestBlockHeight = nodeLogs[0].BlockHeight
	if highestBlockHeight <= 0 {
		logger.Log.WithFields(log.Fields{
			"sessionKey":   options.Session.Key,
			"blockchainID": options.Blockchain,
			"requestID":    sc.RequestID,
			"error":        "sync check: top synced node result is invalid",
		}).Error(errMsg)
	}

	return validNodes, highestBlockHeight
}

// getValidatedAltruist obtains and validates altruist block height and also returns,
// how many nodes are ahead of it
func (sc *SyncChecker) getValidatedAltruist(nodeLogs []*nodeSyncLog, options *SyncCheckOptions) (int64, int) {
	altruistBlockHeight, err := getAltruistBlockHeight(options.SyncCheckOptions, options.AltruistURL, options.SyncCheckOptions.Path)
	if altruistBlockHeight == 0 || err != nil {
		logger.Log.WithFields(log.Fields{
			"sessionKey":   options.Session.Key,
			"blockchainID": options.Blockchain,
			"requestID":    sc.RequestID,
			"serviceNode":  "ALTRUIST",
		}).Error("sync check: altruist failure: ", err)
	}
	logger.Log.WithFields(log.Fields{
		"sessionKey":   options.Session.Key,
		"blockchainID": options.Blockchain,
		"requestID":    sc.RequestID,
		"relayType":    "FALLBACK",
		"blockHeight":  altruistBlockHeight,
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

func getAltruistBlockHeight(options types.SyncCheckOptions, altruistURL string, path string) (int64, error) {
	regex := regexp.MustCompile(`/[\w]*:\/\/[^\/]*@/g`)
	regex.ReplaceAllString(altruistURL, "")

	req, err := http.NewRequest(http.MethodPost, altruistURL+path,
		bytes.NewBuffer([]byte(strings.Replace(options.Body, `\`, "", -1))))

	if err != nil {
		return 0, errors.New("error making altruist request: " + err.Error())
	}
	defer utils.CloseOrLog(req.Response)

	req.Header.Add("Content-Type", "application/json")

	res, err := httpClient.Do(req)
	defer utils.CloseOrLog(res)
	if err != nil {
		return 0, errors.New("error performing altruist request: " + err.Error())
	}

	return utils.ParseIntegerFromPayload(res.Body, options.ResultKey)
}
