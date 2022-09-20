package models

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GatewayAAT struct {
	Version              string `json:"version" bson:"version"`
	Address              string `json:"address" bson:"address"`
	ClientPublicKey      string `json:"clientPublicKey" bson:"clientPublicKey"`
	ApplicationPublicKey string `json:"applicationPublicKey" bson:"applicationPublicKey"`
	ApplicationSignature string `json:"applicationSignature" bson:"applicationSignature"`
}

type GatewaySettings struct {
	WhitelistOrigins    []string `json:"whitelistOrigins" bson:"whitelistOrigins"`
	WhitelistUserAgents []string `json:"whitelistUserAgents" bson:"whitelistUserAgents"`
	SecretKeyRequired   bool     `json:"secretKeyRequired" bson:"secretKeyRequired"`
	SecretKey           string   `json:"secretKey" bson:"secretKey"`
}

type NotificationSettings struct {
	SignedUp      bool `json:"signedUp" bson:"signedUp"`
	Quarter       bool `json:"quarter" bson:"quarter"`
	Half          bool `json:"half" bson:"half"`
	ThreeQuarters bool `json:"threeQuarters" bson:"threeQuarters"`
	Full          bool `json:"full" bson:"full"`
}

type Limits struct {
	PlanType   string `json:"planType" bson:"planType"`
	DailyLimit int    `json:"dailyLimit" bson:"dailyLimit"`
}

type FreeTierApplicationAccount struct {
	Address    string `json:"address" bson:"address"`
	PublicKey  string `json:"publicKey" bson:"publicKey"`
	PrivateKey string `json:"privateKey" bson:"privateKey"`
	PassPhrase string `json:"passPhrase" bson:"passPhrase"`
}

// Application is the schema of the Application data
type Application struct {
	ID                         primitive.ObjectID         `json:"_id" bson:"_id"`
	GatewaySettings            GatewaySettings            `json:"gatewaySettings" bson:"gatewaySettings"`
	Name                       string                     `json:"name" bson:"name"`
	User                       string                     `json:"user" bson:"user"`
	Status                     string                     `json:"status" bson:"status"`
	FreeTier                   bool                       `json:"freeTier" bson:"freeTier"`
	FreeTierApplicationAccount FreeTierApplicationAccount `json:"freeTierApplicationAccount" bson:"freeTierApplicationAccount"`
	GatewayAAT                 GatewayAAT                 `json:"gatewayAAT" bson:"gatewayAAT"`
	Limits                     Limits                     `json:"limits" bson:"limits"`
	NotificationSettings       NotificationSettings       `json:"notificationSettings" bson:"notificationSettings"`
	Dummy                      bool                       `json:"dummy" bson:"dummy"`
}

// ApplicationStore is the interface for all the operations to retrieve data of applications
type ApplicationStore interface {
	GetStakedApplications(ctx context.Context) ([]*Application, error)
	GetGigastakedApplications(ctx context.Context) ([]*Application, error)
	GetSettlersApplications(ctx context.Context) ([]*Application, error)
	GetAppsFromList(ctx context.Context, appIDs []string) ([]*Application, error)
	GetApplications(ctx context.Context) ([]*Application, error)
}
