package gateway

import "context"

type Blockchain struct {
	ID               string `json:"_id"`
	Ticker           string `json:"ticker"`
	ChainID          string `json:"chainID"`
	Network          string `json:"network"`
	Description      string `json:"description"`
	Index            int    `json:"index"`
	Blockchain       string `json:"blockchain"`
	Active           bool   `json:"active"`
	EnforceResult    string `json:"enforceResult"`
	ChainIDCheck     string `json:"chainIDCheck"`
	SyncCheck        string `json:"syncCheck"`
	SyncAllowance    int    `json:"syncAllowance"`
	LogLimitBlocks   int    `json:"logLimitBlocks"`
	SyncCheckOptions struct {
		Body      string `json:"body"`
		ResultKey string `json:"resultKey"`
		Allowance int    `json:"allowance"`
	} `json:"syncCheckOptions"`
	BlockchainAliases []string `json:"blockchainAliases"`
	Redirects         []struct {
		Alias          string `json:"alias"`
		Domain         string `json:"domain"`
		LoadBalancerID string `json:"loadBalancerID"`
	} `json:"redirects"`
	Altruist string `json:"altruist"`
}

type BlockchainStore interface {
	GetBlockchains(ctx context.Context) ([]*Blockchain, error)
}