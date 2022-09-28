package models

import (
	"context"

	"github.com/pokt-foundation/portal-api-go/repository"
)

// Redirects represent the data of a blockchain redirect
type Redirects struct {
	Alias          string `json:"alias"`
	Domain         string `json:"domain"`
	LoadBalancerID string `json:"loadBalancerID"`
}

// SyncCheckOptions is the data needed to perform a sync check
type SyncCheckOptions struct {
	Body      string `json:"body"`
	ResultKey string `json:"resultKey"`
	Path      string `json:"path"`
	Allowance int    `json:"allowance"`
}

// Blockchain is the schema of the Blockchain data
type Blockchain struct {
	ID                string           `json:"_id" bson:"_id"`
	Ticker            string           `json:"ticker" bson:"ticker"`
	ChainID           string           `json:"chainID" bson:"chainID"`
	Network           string           `json:"network" bson:"network"`
	Description       string           `json:"description" bson:"description"`
	Blockchain        string           `json:"blockchain" bson:"blockchain"`
	EnforceResult     string           `json:"enforceResult" bson:"enforceResult"`
	ChainIDCheck      string           `json:"chainIDCheck" bson:"chainIDCheck"`
	SyncCheck         string           `json:"syncCheck" bson:"syncCheck"`
	Altruist          string           `json:"altruist" bson:"altruist"`
	Path              string           `json:"path" bson:"path"`
	Index             int              `json:"index" bson:"index"`
	SyncAllowance     int              `json:"syncAllowance" bson:"syncAllowance"`
	LogLimitBlocks    int              `json:"logLimitBlocks" bson:"logLimitBlocks"`
	Active            bool             `json:"active" bson:"active"`
	BlockchainAliases []string         `json:"blockchainAliases" bson:"blockchainAliases"`
	SyncCheckOptions  SyncCheckOptions `json:"syncCheckOptions" bson:"syncCheckOptions"`
	Redirects         []Redirects      `json:"redirects" bson:"redirects"`
}

// RepositoryToModelBlockchain converts the migrated blockchain struct to one of mongodb
func RepositoryToModelBlockchain(bl *repository.Blockchain) *Blockchain {
	redirects := make([]Redirects, len(bl.Redirects))
	for _, rd := range bl.Redirects {
		redirects = append(redirects, Redirects{
			Alias:          rd.Alias,
			Domain:         rd.Domain,
			LoadBalancerID: rd.LoadBalancerID,
		})
	}

	return &Blockchain{
		ID:             bl.ID,
		Ticker:         bl.Ticker,
		ChainID:        bl.ChainID,
		Network:        bl.Network,
		Description:    bl.Description,
		Blockchain:     bl.Blockchain,
		Active:         bl.Active,
		EnforceResult:  bl.EnforceResult,
		Path:           bl.Path,
		LogLimitBlocks: bl.LogLimitBlocks,
		ChainIDCheck:   bl.ChainIDCheck,
		SyncCheckOptions: SyncCheckOptions{
			Body:      bl.SyncCheckOptions.Body,
			ResultKey: bl.SyncCheckOptions.ResultKey,
			Allowance: bl.SyncCheckOptions.Allowance,
			Path:      bl.SyncCheckOptions.Path,
		},
		Redirects: redirects,
	}
}

// BlockchainStore is the interface for all the operations to retrieve data of blockchains
type BlockchainStore interface {
	GetBlockchains(ctx context.Context) ([]*Blockchain, error)
}
