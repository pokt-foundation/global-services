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
