package pocket

import "time"

type Session struct {
	Header struct {
		AppPublicKey  string `json:"app_public_key"`
		Chain         string `json:"chain"`
		SessionHeight int    `json:"session_height"`
	} `json:"header"`
	Key   string `json:"key"`
	Nodes []struct {
		Address       string    `json:"address"`
		Chains        []string  `json:"chains"`
		Jailed        bool      `json:"jailed"`
		PublicKey     string    `json:"public_key"`
		ServiceURL    string    `json:"service_url"`
		Status        int       `json:"status"`
		Tokens        string    `json:"tokens"`
		UnstakingTime time.Time `json:"unstaking_time"`
	} `json:"nodes"`
}

// SessionCamelCase is a session object with all is fields made to be formatted
// in JSON as camelCase
type SessionCamelCase struct {
	Header struct {
		AppPublicKey  string `json:"applicationPubKey"`
		Chain         string `json:"chain"`
		SessionHeight int    `json:"sessionBlockHeight"`
	} `json:"header"`
	Key   string `json:"key"`
	Nodes []struct {
		Address       string    `json:"address"`
		Chains        []string  `json:"chains"`
		Jailed        bool      `json:"jailed"`
		PublicKey     string    `json:"publicKey"`
		ServiceURL    string    `json:"serviceUrl"`
		Status        int       `json:"status"`
		Tokens        string    `json:"stakedTokens"`
		UnstakingTime time.Time `json:"unstakingTime"`
	} `json:"nodes"`
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
