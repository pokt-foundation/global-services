package main

import (
	"context"
	"errors"
	"net/http"

	postgresgateway "github.com/Pocket/global-services/mongo-migration/postgres_client"
	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/database"
	"github.com/Pocket/global-services/shared/gateway/models"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/pokt-foundation/portal-api-go/repository"
	"github.com/pokt-foundation/utils-go/environment"
)

var (
	postgresClientUrl     = environment.MustGetString("POSTGRES_CLIENT_URL")
	authenticationToken   = environment.MustGetString("AUTHENTICATION_TOKEN")
	mongoConnectionString = environment.MustGetString("MONGO_CONNECTION_STRING")
	mongoDatabase         = environment.GetString("MONGO_DATABASE", "gateway")
)

// LambdaHandler manages the process of getting the postgres apps migrated to mongo
func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	m, p, err := migrateToMongo(ctx)

	if err != nil {
		return *apigateway.NewJSONResponse(http.StatusOK, map[string]any{
			"err": err.Error(),
		}), nil
	}
	return *apigateway.NewJSONResponse(http.StatusOK, map[string]any{
		"m": m,
		"p": p,
	}), nil
}

func main() {
	lambda.Start(LambdaHandler)
}

func migrateToMongo(ctx context.Context) (int, int, error) {
	mongo, err := database.ClientFromURI(ctx, mongoConnectionString, mongoDatabase)
	if err != nil {
		return 0, 0, errors.New("error connecting to mongo: " + err.Error())
	}

	postgres := postgresgateway.NewPostgresClient(postgresClientUrl, authenticationToken)

	mongoApps, postgresApps, err := getApplications(ctx, mongo, postgres)
	if err != nil {
		return 0, 0, err
	}
	mongoLBs, postgresLBs, err := getLoadBalancers(ctx, mongo, postgres)
	if err != nil {
		return 0, 0, err
	}

	lbsNotInMongo := getItemsNotInMongo(postgresLBs, mongoLBs)
	appsNotInMongo := getItemsNotInMongo(postgresApps, mongoApps)

	return len(lbsNotInMongo), len(appsNotInMongo), nil
}

func getApplications(ctx context.Context, mongo *database.Mongo, postgres *postgresgateway.Client) (map[string]*models.Application, map[string]*repository.Application, error) {
	mongoAppsArr, err := mongo.GetApplications(ctx)
	if err != nil {
		return nil, nil, err
	}

	postgresAppsArr, err := postgres.GetAllApplications()
	if err != nil {
		return nil, nil, err
	}

	mongoApps := utils.SliceToMappedStruct(mongoAppsArr, func(app *models.Application) string {
		return app.ID.Hex()
	})

	postgresApps := utils.SliceToMappedStruct(postgresAppsArr, func(app *repository.Application) string {
		return app.ID
	})

	return mongoApps, postgresApps, nil
}

func getLoadBalancers(ctx context.Context, mongo *database.Mongo, postgres *postgresgateway.Client) (map[string]*models.LoadBalancer, map[string]*repository.LoadBalancer, error) {
	mongoLBsArr, err := mongo.GetLoadBalancers(ctx)
	if err != nil {
		return nil, nil, err
	}

	postgresLBsArr, err := postgres.GetAllLoadBalancers()
	if err != nil {
		return nil, nil, err
	}

	mongoLBs := utils.SliceToMappedStruct(mongoLBsArr, func(lb *models.LoadBalancer) string {
		return lb.ID.Hex()
	})

	postgresLBs := utils.SliceToMappedStruct(postgresLBsArr, func(lb *repository.LoadBalancer) string {
		return lb.ID
	})

	return mongoLBs, postgresLBs, nil
}

func getItemsNotInMongo[T any, V any](pgItems map[string]*T, mongoItems map[string]*V) []*T {
	items := make([]*T, 0)
	for key, item := range pgItems {
		_, ok := mongoItems[key]
		if ok {
			continue
		}
		items = append(items, item)
	}
	return items
}

func postgresAppToMongoApp(pgApps []*repository.Application) ([]*models.Application, error) {
	mongoApps := make([]*models.Application, 0)

	for _, pgApp := range pgApps {
		id, err := primitive.ObjectIDFromHex(pgApp.ID)
		if err != nil {
			return nil, err
		}
		mongoApps = append(mongoApps, &models.Application{
			ID:    id,
			Dummy: pgApp.Dummy,
			GatewayAAT: models.GatewayAAT{
				Address:              pgApp.GatewayAAT.Address,
				ClientPublicKey:      pgApp.GatewayAAT.ClientPublicKey,
				ApplicationPublicKey: pgApp.GatewayAAT.ApplicationPublicKey,
				ApplicationSignature: pgApp.GatewayAAT.ApplicationSignature,
				Version:              "0.0.1",
			},
			GatewaySettings: models.GatewaySettings{
				WhitelistOrigins:    pgApp.GatewaySettings.WhitelistOrigins,
				WhitelistUserAgents: pgApp.GatewaySettings.WhitelistUserAgents,
				SecretKey:           pgApp.GatewaySettings.SecretKey,
				SecretKeyRequired:   pgApp.GatewaySettings.SecretKeyRequired,
			},
			NotificationSettings: models.NotificationSettings{
				SignedUp:      pgApp.NotificationSettings.SignedUp,
				Quarter:       pgApp.NotificationSettings.Quarter,
				Half:          pgApp.NotificationSettings.Half,
				ThreeQuarters: pgApp.NotificationSettings.ThreeQuarters,
				Full:          pgApp.NotificationSettings.Full,
			},
			Limits: models.Limits{
				PlanType:   string(pgApp.Limits.PlanType),
				DailyLimit: pgApp.Limits.DailyLimit,
			},
			FreeTierApplicationAccount: models.FreeTierApplicationAccount{
				Address:   pgApp.GatewayAAT.Address,
				PublicKey: pgApp.GatewayAAT.ClientPublicKey,
			},
		})
	}

	return mongoApps, nil
}
