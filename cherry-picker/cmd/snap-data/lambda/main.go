package main

import (
	"context"
	"net/http"

	cpicker "github.com/Pocket/global-services/cherry-picker"
	snapdata "github.com/Pocket/global-services/cherry-picker/cmd/snap-data"
	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"

	postgresdriver "github.com/Pocket/global-services/cherry-picker/database"
	logger "github.com/Pocket/global-services/shared/logger"
	log "github.com/sirupsen/logrus"
)

func lambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	snapCherryPickerData := &snapdata.SnapCherryPicker{
		RequestID: lc.AwsRequestID,
	}
	snapCherryPickerData.Regions = make(map[string]*snapdata.Region)
	defer clean(snapCherryPickerData)

	err := snapCherryPickerData.Init(ctx)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": snapCherryPickerData.RequestID,
			"error":     err.Error(),
		}).Error("error initializing:", err.Error())
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	snapCherryPickerData.SnapCherryPickerData(ctx)

	return *apigateway.NewJSONResponse(http.StatusOK, map[string]interface{}{
		"ok": true,
	}), err
}

func main() {
	lambda.Start(lambdaHandler)
}

func clean(sn *snapdata.SnapCherryPicker) {
	utils.RunFnOnSliceSingleFailure(sn.Stores, func(store cpicker.CherryPickerStore) error {
		switch str := store.(type) {
		case *postgresdriver.CherryPickerPostgres:
			str.Db.Conn.Close()
		}
		return nil
	})

	for _, cache := range sn.Caches {
		cache.Close()
	}
}
