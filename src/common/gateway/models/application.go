package models

import (
	"context"
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

type ApplicationStore interface {
	GetStakedApplications(ctx context.Context) ([]*Application, error)
	GetGigastakedApplications(ctx context.Context) ([]*Application, error)
	GetSettlersApplications(ctx context.Context) ([]*Application, error)
}
