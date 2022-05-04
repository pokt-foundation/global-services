package base

import (
	"github.com/Pocket/global-services/src/common/gateway/models"
	"github.com/pokt-foundation/pocket-go/provider"
)

// Payload is the data needed for the perform-application-check to work
type Payload struct {
	Session                provider.Session   `json:"session"`
	Blockchain             models.Blockchain  `json:"blockchain"`
	AAT                    provider.PocketAAT `json:"aat"`
	DefaultAllowance       int                `json:"defaultAllowance"`
	AltruistTrustThreshold float32            `json:"altruistTrustThreshold"`
	RequestID              string             `json:"requestID"`
}

// Response represents the output of the perform-application-check lambda
type Response struct {
	SyncCheckedNodes  map[string][]string `json:"syncCheckedNodes"`
	ChainCheckedNodes map[string][]string `json:"chainCheckedNodes"`
}
