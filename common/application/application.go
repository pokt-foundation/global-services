package common

import (
	"context"
	"time"
)

type Application struct {
	ID              string `json:"_id"`
	GatewaySettings struct {
		WhitelistOrigins    []interface{} `json:"whitelistOrigins"`
		WhitelistUserAgents []interface{} `json:"whitelistUserAgents"`
		SecretKeyRequired   bool          `json:"secretKeyRequired"`
		SecretKey           string        `json:"secretKey"`
	} `json:"gatewaySettings"`
	Name                       string `json:"name"`
	User                       string `json:"user"`
	Status                     string `json:"status"`
	FreeTier                   bool   `json:"freeTier"`
	FreeTierApplicationAccount struct {
		Address    string `json:"address"`
		PublicKey  string `json:"publicKey"`
		PrivateKey string `json:"privateKey"`
		PassPhrase string `json:"passPhrase"`
	} `json:"freeTierApplicationAccount"`
	GatewayAAT struct {
		Version              string `json:"version"`
		ClientPublicKey      string `json:"clientPublicKey"`
		ApplicationPublicKey string `json:"applicationPublicKey"`
		ApplicationSignature string `json:"applicationSignature"`
	} `json:"gatewayAAT"`
	NotificationSettings struct {
		SignedUp      bool `json:"signedUp"`
		Quarter       bool `json:"quarter"`
		Half          bool `json:"half"`
		ThreeQuarters bool `json:"threeQuarters"`
		Full          bool `json:"full"`
	} `json:"notificationSettings"`
	Dummy bool `json:"dummy"`
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

type ApplicationStore interface {
	GetAllStakedApplications(ctx context.Context) ([]*Application, error)
}
