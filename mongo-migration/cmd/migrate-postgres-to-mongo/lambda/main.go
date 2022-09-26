package main

import (
	"context"
	"errors"
	"net/http"

	postgresgateway "github.com/Pocket/global-services/mongo-migration/postgres_client"
	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/database"
	"github.com/Pocket/global-services/shared/gateway/models"
	logger "github.com/Pocket/global-services/shared/logger"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	log "github.com/sirupsen/logrus"

	"github.com/pokt-foundation/portal-api-go/repository"
	"github.com/pokt-foundation/utils-go/environment"
)

var (
	postgresClientURL     = environment.MustGetString("POSTGRES_CLIENT_URL")
	authenticationToken   = environment.MustGetString("AUTHENTICATION_TOKEN")
	mongoConnectionString = environment.MustGetString("MONGO_CONNECTION_STRING")
	mongoDatabase         = environment.GetString("MONGO_DATABASE", "gateway")
)

// LambdaHandler manages the process of getting the postgres apps migrated to mongo
func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	appsCount, lbsCount, err := migrateToMongo(ctx)

	if err != nil {
		return *apigateway.NewJSONResponse(http.StatusOK, map[string]any{
			"err": err.Error(),
		}), nil
	}
	return *apigateway.NewJSONResponse(http.StatusOK, map[string]any{
		"newApps": appsCount,
		"newLbs":  lbsCount,
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

	postgres := postgresgateway.NewPostgresClient(postgresClientURL, authenticationToken)

	mongoApps, postgresApps, err := getApplications(ctx, mongo, postgres)
	if err != nil {
		return 0, 0, err
	}
	mongoLBs, postgresLBs, err := getLoadBalancers(ctx, mongo, postgres)
	if err != nil {
		return 0, 0, err
	}

	appsToWrite := convertRepositoryToMongo(getItemsNotInMongo(postgresApps, mongoApps), models.RepositoryToModelApp)
	lbsToWrite := convertRepositoryToMongo(getItemsNotInMongo(postgresLBs, mongoLBs), models.RepositoryToModelLoadBalancer)

	if len(appsToWrite) > 0 {
		err = mongo.InsertMany(ctx, database.ApplicationCollection, appsToWrite)
		if err != nil {
			return 0, 0, err
		}
	}

	if len(lbsToWrite) > 0 {
		err = mongo.InsertMany(ctx, database.LoadBalancersCollection, lbsToWrite)
		if err != nil {
			return 0, 0, err
		}
	}

	return len(appsToWrite), len(lbsToWrite), nil
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

func getItemsNotInMongo[T any, V any](pgItems map[string]*T, mongoItems map[string]*V) []T {
	items := make([]T, 0)
	for key, item := range pgItems {
		_, ok := mongoItems[key]
		if ok {
			continue
		}
		items = append(items, *item)
	}
	return items
}

func convertRepositoryToMongo[T any, V any](items []T, convertFn func(*T) (V, error)) []any {
	convertedItems := make([]any, 0)
	for _, item := range items {
		convertedItem, err := convertFn(&item)
		if err != nil {
			logger.Log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("could not convert repository item to a mongo type")
			continue
		}

		convertedItems = append(convertedItems, convertedItem)
	}

	return convertedItems
}
