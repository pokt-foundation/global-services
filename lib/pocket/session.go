package pocket

import (
	"time"

	"github.com/pokt-foundation/pocket-go/pkg/provider"
)

// Session is a session object with all is fields made to be formatted
// in JSON as camelCase
type Session struct {
	// BlockHeight is not actual part of the provider's session struct, is here
	// for convenience to save the actual blockheight when the session was dispatched
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
		Key: session.Key,
		Header: &SesionHeader{
			AppPublicKey:  session.Header.AppPublicKey,
			Chain:         session.Header.Chain,
			SessionHeight: session.Header.SessionHeight,
		},
		Nodes: nodes,
	}
}

// ToProviderSession converts the pocket Session back to one from the provider package
func (s *Session) ToProviderSession() *provider.Session {
	if s == nil {
		return nil
	}

	nodes := []*provider.Node{}

	for _, node := range s.Nodes {
		nodes = append(nodes, &provider.Node{
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

	return &provider.Session{
		Key: s.Key,
		Header: &provider.SessionHeader{
			AppPublicKey:  s.Header.AppPublicKey,
			SessionHeight: s.Header.SessionHeight,
			Chain:         s.Header.Chain,
		},
		Nodes: nodes,
	}
}
