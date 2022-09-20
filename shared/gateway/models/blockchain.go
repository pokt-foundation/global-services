package models

import (
	"context"
)

// Blockchain is the schema of the Blockchain data
type Blockchain struct {
	ID                string           `json:"_id"`
	Ticker            string           `json:"ticker"`
	ChainID           string           `json:"chainID"`
	Network           string           `json:"network"`
	Description       string           `json:"description"`
	Index             int              `json:"index"`
	Blockchain        string           `json:"blockchain"`
	Active            bool             `json:"active"`
	EnforceResult     string           `json:"enforceResult"`
	ChainIDCheck      string           `json:"chainIDCheck"`
	SyncCheck         string           `json:"syncCheck"`
	SyncAllowance     int              `json:"syncAllowance"`
	LogLimitBlocks    int              `json:"logLimitBlocks"`
	Path              string           `json:"path"`
	SyncCheckOptions  SyncCheckOptions `json:"syncCheckOptions"`
	BlockchainAliases []string         `json:"blockchainAliases"`
	Redirects         []struct {
		Alias          string `json:"alias"`
		Domain         string `json:"domain"`
		LoadBalancerID string `json:"loadBalancerID"`
	} `json:"redirects"`
	Altruist string `json:"altruist"`
}

// SyncCheckOptions is the data needed to perform a sync check
type SyncCheckOptions struct {
	Body      string `json:"body"`
	ResultKey string `json:"resultKey"`
	Path      string `json:"path"`
	Allowance int    `json:"allowance"`
}

// BlockchainStore is the interface for all the operations to retrieve data of blockchains
type BlockchainStore interface {
	GetBlockchains(ctx context.Context) ([]*Blockchain, error)
}
