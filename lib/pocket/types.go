package pocket

import (
	"time"

	"github.com/pokt-foundation/pocket-go/pkg/provider"
)

// Session is a session object with all is fields made to be formatted
// in JSON as camelCase
type Session struct {
	BlockHeight int           `json:"blockHeight"`
	Header      *SesionHeader `json:"header"`
	Key         string        `json:"key"`
	Nodes       []*Node       `json:"nodes"`
}

type Node struct {
	Address       string    `json:"address"`
	Chains        []string  `json:"chains"`
	Jailed        bool      `json:"jailed"`
	PublicKey     string    `json:"publicKey"`
	ServiceURL    string    `json:"serviceUrl"`
	Status        int       `json:"status"`
	Tokens        string    `json:"stakedTokens"`
	UnstakingTime time.Time `json:"unstakingTime"`
}

type SesionHeader struct {
	AppPublicKey  string `json:"applicationPubKey"`
	Chain         string `json:"chain"`
	SessionHeight int    `json:"sessionBlockHeight"`
}

// NewSessionCamelCase returns a Session-like struct with the tags as camelcase
func NewSessionCamelCase(session *provider.Session) *Session {
	nodes := []*Node{}

	for _, node := range session.Nodes {
		nodes = append(nodes, &Node{
			Address:       node.Address,
			Chains:        node.Chains,
			Jailed:        node.Jailed,
			PublicKey:     node.PublicKey,
			ServiceURL:    node.ServiceURL,
			Status:        node.Status,
			Tokens:        node.Tokens,
			UnstakingTime: node.UnstakingTime,
		})
	}

	return &Session{
		BlockHeight: session.BlockHeight,
		Key:         session.Key,
		Header: &SesionHeader{
			AppPublicKey:  session.Header.AppPublicKey,
			Chain:         session.Header.Chain,
			SessionHeight: session.Header.SessionHeight,
		},
		Nodes: nodes,
	}
}

type NetworkApplication struct {
	Address       string    `json:"address"`
	PublicKey     string    `json:"public_key"`
	Jailed        bool      `json:"jailed"`
	Chains        []string  `json:"chains"`
	MaxRelays     string    `json:"max_relays"`
	Status        int       `json:"status"`
	StakedTokens  string    `json:"staked_tokens"`
	UnstakingTime time.Time `json:"unstaking_time"`
}
