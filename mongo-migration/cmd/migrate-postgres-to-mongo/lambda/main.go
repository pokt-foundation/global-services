package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	postgresclient "github.com/Pocket/global-services/mongo-migration/postgres_client"
	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/database"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

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

	postgres := postgresclient.NewPostgresClient(postgresClientUrl, authenticationToken)

	mongoApps, err := mongo.GetApplications(ctx)
	if err != nil {
		return 0, 0, err
	}

	postgresApps, err := postgres.GetAllApplications()
	if err != nil {
		return 0, 0, err
	}

	fmt.Println(len(mongoApps), len(postgresApps))

	return len(mongoApps), len(postgresApps), nil
}
