// TODO: Currently this only works for ethereum, add support for more chains
package pocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	logger "github.com/Pocket/global-services/shared/logger"
	"github.com/Pocket/global-services/shared/metrics"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/pokt-foundation/pocket-go/provider"
	"github.com/pokt-foundation/pocket-go/relayer"
	log "github.com/sirupsen/logrus"
)

const (
	mergeBlockNumber = 15537351
	// TODO: Change number
	terminalTotalDifficulty = "0xC70D808A128D7380000"
	mergeCheckPayload       = `{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",false],"id":1}`
)

type ethGetBlockByNumberResult struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		BaseFeePerGas    string        `json:"baseFeePerGas"`
		Difficulty       string        `json:"difficulty"`
		ExtraData        string        `json:"extraData"`
		GasLimit         string        `json:"gasLimit"`
		GasUsed          string        `json:"gasUsed"`
		Hash             string        `json:"hash"`
		Miner            string        `json:"miner"`
		LogsBloom        string        `json:"logsBloom"`
		MixHash          string        `json:"mixHash"`
		Nonce            string        `json:"nonce"`
		Number           string        `json:"number"`
		ParentHash       string        `json:"parentHash"`
		ReceiptsRoot     string        `json:"receiptsRoot"`
		Sha3Uncles       string        `json:"sha3Uncles"`
		Size             string        `json:"size"`
		StateRoot        string        `json:"stateRoot"`
		Timestamp        string        `json:"timestamp"`
		TotalDifficulty  string        `json:"totalDifficulty"`
		Transactions     []string      `json:"transactions"`
		TransactionsRoot string        `json:"transactionsRoot"`
		Uncles           []interface{} `json:"uncles"`
	} `json:"result"`
}

// ChainChecker is the struct to perform merge checks on app sessions
type MergeChecker struct {
	Relayer         *relayer.Relayer
	MetricsRecorder *metrics.Recorder
	RequestID       string
}

// MergeCheckOptions is the struct of the data needed to perform a merge check
type MergeCheckOptions struct {
	Session    provider.Session
	PocketAAT  provider.PocketAAT
	Blockchain string
	ChainID    string
	Path       string
}

type nodeMergeLog struct {
	Node        *provider.Node
	Difficulty  string
	BlockNumber int64
}

func (mc *MergeChecker) Check(ctx context.Context, options MergeCheckOptions) []string {
	mergedNodes := []string{}
	// if (nodeDifficulty === TERMINAL_TOTAL_DIFFICULTY && nodeBlockNumber > MERGE_BLOCK_NUMBER) {

	nodeLogs := mc.getNodeMergeLogs(ctx, &options)

	for _, node := range nodeLogs {
		publicKey := node.Node.PublicKey
		logData := log.Fields{
			"sessionKey":            options.Session.Key,
			"blockhainID":           options.Blockchain,
			"requestID":             mc.RequestID,
			"serviceURL":            node.Node.ServiceURL,
			"serviceDomain":         utils.GetDomainFromURL(node.Node.ServiceURL),
			"serviceNode":           node.Node.PublicKey,
			"appplicationPublicKey": options.Session.Header.AppPublicKey,
		}

		if node.Difficulty != terminalTotalDifficulty || node.BlockNumber < mergeBlockNumber {
			logger.Log.WithFields(logData).Warn(fmt.Sprintf("MERGE CHECK FAILURE: %s difficulty: %s block number: %d", publicKey, node.Difficulty, node.BlockNumber))
			continue
		}

		logger.Log.WithFields(logData).Info(fmt.Sprintf("MERGE CHECK SUCCES: %s difficulty: %s block number: %d", publicKey, node.Difficulty, node.BlockNumber))

		mergedNodes = append(mergedNodes, publicKey)
	}

	logger.Log.WithFields(log.Fields{
		"sessionKey":            options.Session.Key,
		"blockhainID":           options.Blockchain,
		"requestID":             mc.RequestID,
		"appplicationPublicKey": options.Session.Header.AppPublicKey,
	}).Info(fmt.Sprintf("MERGE CHECK COMPLETE: %d nodes merged", len(mergedNodes)))

	return mergedNodes
}

func (mc *MergeChecker) getNodeMergeLogs(ctx context.Context, options *MergeCheckOptions) []*nodeMergeLog {
	nodeLogsChan := make(chan *nodeMergeLog, len(options.Session.Nodes))
	nodeLogs := []*nodeMergeLog{}

	var wg sync.WaitGroup
	for _, node := range options.Session.Nodes {
		wg.Add(1)
		go func(n *provider.Node) {
			defer wg.Done()
			mc.getNodeMergeLog(ctx, n, nodeLogsChan, options)
		}(node)
		wg.Wait()
	}

	close(nodeLogsChan)

	for log := range nodeLogsChan {
		nodeLogs = append(nodeLogs, log)
	}

	return nodeLogs
}

func (mc *MergeChecker) getNodeMergeLog(ctx context.Context, node *provider.Node, nodeLogs chan<- *nodeMergeLog, options *MergeCheckOptions) {
	relay, err := mc.Relayer.Relay(&relayer.Input{
		Blockchain: options.Blockchain,
		Data:       mergeCheckPayload,
		Method:     http.MethodPost,
		PocketAAT:  &options.PocketAAT,
		Session:    &options.Session,
		Node:       node,
		Path:       options.Path,
	}, nil)

	logData := log.Fields{
		"sessionKey":    options.Session.Key,
		"blockhainID":   options.Blockchain,
		"requestID":     mc.RequestID,
		"serviceURL":    node.ServiceURL,
		"serviceDomain": utils.GetDomainFromURL(node.ServiceURL),
		"serviceNode":   node.PublicKey,
		"error":         err.Error(),
	}
	if err != nil {
		logger.Log.WithFields(logData).Error("merge check: error obtaining merge data: ", err)
	}

	var relayResult ethGetBlockByNumberResult
	if err = json.Unmarshal([]byte(relay.RelayOutput.Response), &relayResult); err != nil {
		logger.Log.WithFields(logData).Error("merge check: error unmarshalling merge data: ", err)
	}

	blockNumber, err := strconv.ParseInt(relayResult.Result.Number, 16, 64)
	if err != nil {
		logger.Log.WithFields(logData).Error("merge check: error converting block number from hex to int: ", err)
	}

	nodeLogs <- &nodeMergeLog{
		Node:        node,
		Difficulty:  relayResult.Result.Difficulty,
		BlockNumber: blockNumber,
	}

}
