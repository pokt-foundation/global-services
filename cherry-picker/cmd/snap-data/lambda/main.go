package main

import (
	"context"
	"net/http"

	snapdata "github.com/Pocket/global-services/cherry-picker/cmd/snap-data"
	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"

	logger "github.com/Pocket/global-services/shared/logger"
	log "github.com/sirupsen/logrus"
)

func LambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	snapCherryPickerData := &snapdata.SnapCherryPicker{
		RequestID: lc.AwsRequestID,
	}
	snapCherryPickerData.Regions = make(map[string]*snapdata.Region)

	err := snapCherryPickerData.Init(ctx)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": snapCherryPickerData.RequestID,
			"error":     err.Error(),
		}).Error("error initializing:", err.Error())
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	err = snapCherryPickerData.SnapCherryPickerData(ctx)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": snapCherryPickerData.RequestID,
			"error":     err.Error(),
		}).Error("error getting cherry picker data:", err.Error())
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(http.StatusOK, map[string]interface{}{
		"ok": true,
	}), err
}

func main() {
	lambda.Start(LambdaHandler)
}
