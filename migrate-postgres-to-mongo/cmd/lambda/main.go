package main

import (
	"context"
	"net/http"

	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// LambdaHandler manages the process of getting the postgres apps migrated to mongo
func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {

	return *apigateway.NewJSONResponse(http.StatusOK, nil), nil
}

func main() {
	lambda.Start(LambdaHandler)
}
