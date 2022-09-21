package models

import (
	"context"

	"github.com/pokt-foundation/portal-api-go/repository"
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

func RepositoryToModelApp(app *repository.Application) (*Application, error) {
	id := primitive.NewObjectID()
	var err error
	if app.ID != "" {
		id, err = primitive.ObjectIDFromHex(app.ID)
		if err != nil {
			return nil, err
		}
	}

	return &Application{
		ID:    id,
		Name:  app.Name,
		User:  app.UserID,
		Dummy: app.Dummy,
		GatewayAAT: GatewayAAT{
			Address:              app.GatewayAAT.Address,
			ClientPublicKey:      app.GatewayAAT.ClientPublicKey,
			ApplicationPublicKey: app.GatewayAAT.ApplicationPublicKey,
			ApplicationSignature: app.GatewayAAT.ApplicationSignature,
			Version:              "0.0.1",
		},
		GatewaySettings: GatewaySettings{
			WhitelistOrigins:    app.GatewaySettings.WhitelistOrigins,
			WhitelistUserAgents: app.GatewaySettings.WhitelistUserAgents,
			SecretKey:           app.GatewaySettings.SecretKey,
			SecretKeyRequired:   app.GatewaySettings.SecretKeyRequired,
		},
		NotificationSettings: NotificationSettings{
			SignedUp:      app.NotificationSettings.SignedUp,
			Quarter:       app.NotificationSettings.Quarter,
			Half:          app.NotificationSettings.Half,
			ThreeQuarters: app.NotificationSettings.ThreeQuarters,
			Full:          app.NotificationSettings.Full,
		},
		Limits: Limits{
			PlanType:   string(app.Limits.PlanType),
			DailyLimit: app.Limits.DailyLimit,
		},
		FreeTierApplicationAccount: FreeTierApplicationAccount{
			Address:   app.GatewayAAT.Address,
			PublicKey: app.GatewayAAT.ClientPublicKey,
		},
	}, nil
}

// ApplicationStore is the interface for all the operations to retrieve data of applications
type ApplicationStore interface {
	GetStakedApplications(ctx context.Context) ([]*Application, error)
	GetGigastakedApplications(ctx context.Context) ([]*Application, error)
	GetSettlersApplications(ctx context.Context) ([]*Application, error)
	GetAppsFromList(ctx context.Context, appIDs []string) ([]*Application, error)
	GetApplications(ctx context.Context) ([]*Application, error)
}
