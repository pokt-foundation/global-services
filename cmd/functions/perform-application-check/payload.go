package base

import (
	"github.com/Pocket/global-dispatcher/common/gateway/models"
	"github.com/pokt-foundation/pocket-go/pkg/provider"
)

type Payload struct {
	Session                provider.Session   `json:"session"`
	Blockchain             models.Blockchain  `json:"blockchain"`
	AAT                    provider.PocketAAT `json:"aat"`
	DefaultAllowance       int                `json:"defaultAllowance"`
	AltruistTrustThreshold float32            `json:"altruistTrustThreshold"`
	RequestID              string             `json:"requestID"`
}

type Response struct {
	SyncCheckedNodes  []string `json:"syncCheckedNodes"`
	ChainCheckedNodes []string `json:"chainCheckedNodes"`
}
