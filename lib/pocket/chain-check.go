package pocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/pokt-foundation/pocket-go/pkg/provider"
	"github.com/pokt-foundation/pocket-go/pkg/relayer"
)

type ChainChecker struct {
	Relayer    *relayer.PocketRelayer
	CommitHash string
}

type ChainCheckOptions struct {
	Session    *provider.Session
	Blockchain string
	Data       string
	ChainID    string
	Path       string
	PocketAAT  *provider.PocketAAT
}

type NodeChainLog struct {
	Node  *provider.Node
	Chain string
}

func (ch *ChainChecker) Check(options *ChainCheckOptions) []string {
	checkedNodes := []string{}
	nodeLogs := ch.GetNodeChainLogs(options)

	for _, node := range nodeLogs {
		publicKey := node.Node.PublicKey
		if node.Chain != options.ChainID {
			// TODO Better logging
			fmt.Println("CHAIN CHECK FAILURE", publicKey)
			continue
		}

		// TODO: Better logging
		fmt.Println("CHAIN CHECK SUCCESS", publicKey)

		checkedNodes = append(checkedNodes, publicKey)
	}

	// TODO: Better logging
	fmt.Printf("CHAIN CHECK COMPLETE: %d nodes on chain", len(checkedNodes))

	// TODO: Implement challenge

	return checkedNodes
}

func (ch *ChainChecker) GetNodeChainLogs(options *ChainCheckOptions) []*NodeChainLog {
	nodeLogsChan := make(chan *NodeChainLog, len(options.Session.Nodes))
	nodeLogs := []*NodeChainLog{}

	var wg sync.WaitGroup
	for _, node := range options.Session.Nodes {
		wg.Add(1)
		go func(n *provider.Node) {
			defer wg.Done()
			ch.GetNodeChainLog(n, nodeLogsChan, options)
		}(node)
	}
	wg.Wait()

	close(nodeLogsChan)

	for log := range nodeLogsChan {
		nodeLogs = append(nodeLogs, log)
	}

	return nodeLogs
}

func (ch *ChainChecker) GetNodeChainLog(node *provider.Node, nodeLogs chan<- *NodeChainLog, options *ChainCheckOptions) {
	relay, err := ch.Relayer.Relay(&relayer.RelayInput{
		Blockchain: options.Blockchain,
		Data:       strings.Replace(options.Data, `\`, "", -1),
		Method:     http.MethodPost,
		PocketAAT:  options.PocketAAT,
		Session:    options.Session,
		Node:       node,
	}, &provider.RelayRequestOptions{})

	if err != nil {
		// TODO: Handle errors
		fmt.Println("error relaying:", err)
		nodeLogs <- &NodeChainLog{
			Node:  node,
			Chain: "0",
		}
		return
	}

	var chainIDHex struct {
		Result string `json:"result"`
	}
	err = json.Unmarshal([]byte(relay.Response.Response), &chainIDHex)
	if err != nil {
		// TODO: Handle errors better
		fmt.Println("error unmarshalling relay response:", err)
		nodeLogs <- &NodeChainLog{
			Node:  node,
			Chain: "0",
		}
		return
	}

	chainIDDecimal, err := strconv.ParseInt(chainIDHex.Result, 0, 64)
	if err != nil {
		// TODO: Handle errors better
		fmt.Println("err converting chainID from hex to decimal:", err)
		nodeLogs <- &NodeChainLog{
			Node:  node,
			Chain: "0",
		}
		return
	}

	nodeLogs <- &NodeChainLog{
		Node:  node,
		Chain: strconv.Itoa(int(chainIDDecimal)),
	}
}
