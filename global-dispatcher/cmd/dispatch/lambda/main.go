package main

import (
	"context"
	"net/http"

	base "github.com/Pocket/global-services/global-dispatcher/cmd/dispatch-globally"
	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"

	logger "github.com/Pocket/global-services/shared/logger"
	log "github.com/sirupsen/logrus"
)

// LambdaHandler manages the DispatchSession call to return as an APIGatewayProxyResponse
func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)
	var statusCode int

	failedDispatcherCalls, err := base.DispatchSessions(ctx, lc.AwsRequestID)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": lc.AwsRequestID,
			"error":     err.Error(),
		}).Error("ERROR DISPATCHING SESSION: " + err.Error())
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	result := map[string]interface{}{
		"ok":                    true,
		"failedDispatcherCalls": failedDispatcherCalls,
	}

	// Internal logging
	logger.Log.WithFields(result).Info("GLOBAL DISPATCHER RESULT")

	return *apigateway.NewJSONResponse(statusCode, result), err
}

func main() {
	lambda.Start(LambdaHandler)
}
