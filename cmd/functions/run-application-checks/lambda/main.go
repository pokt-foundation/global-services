package main

import (
	"context"
	"encoding/json"
	"net/http"

	base "github.com/Pocket/global-dispatcher/cmd/functions/run-application-checks"
	"github.com/Pocket/global-dispatcher/common/apigateway"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"

	performAppCheck "github.com/Pocket/global-dispatcher/cmd/functions/perform-application-check"
	awsLambda "github.com/aws/aws-sdk-go/service/lambda"

	logger "github.com/Pocket/global-dispatcher/lib/logger"
	log "github.com/sirupsen/logrus"
)

var (
	region                   = environment.GetString("AWS_REGION", "")
	performCheckFunctionName = environment.GetString("PERFORM_CHECK_FUNCTION_NAME", "")

	sess = session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client = awsLambda.New(sess, &aws.Config{
		Region: aws.String(region)})
)

func lambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	var statusCode int

	err := base.RunApplicationChecks(ctx, lc.AwsRequestID, PerformChecks)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": lc.AwsRequestID,
			"error":     err.Error(),
		}).Error("ERROR RUNNING APPLICATION CHECKS: " + err.Error())
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(statusCode, map[string]interface{}{
		"ok": true,
	}), err
}

func PerformChecks(ctx context.Context, options *base.PerformChecksOptions) {
	options.Sem.Acquire(ctx, 2)
	defer options.Sem.Release(2)
	options.Wg.Add(1)
	defer options.Wg.Done()

	nodes, err := invokeChecks(options)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID":    options.Ac.RequestID,
			"error":        err.Error(),
			"blockchainID": options.Blockchain.ID,
			"sessionKey":   options.Session.Key,
		}).Error("perform checks: error invoking check: " + err.Error())
		return
	}

	if options.Blockchain.ChainIDCheck != "" {
		if err = base.CacheNodes(nodes.ChainCheckedNodes, options.Ac.CacheBatch, options.ChainCheckKey, options.CacheTTL); err != nil {
			logger.Log.WithFields(log.Fields{
				"error":        err.Error(),
				"requestID":    options.Ac.RequestID,
				"blockchainID": options.Blockchain.ID,
				"sessionKey":   options.Session.Key,
			}).Error("perform checks: error caching chain check nodes: " + err.Error())
		}
	}
	syncCheckOptions := options.Blockchain.SyncCheckOptions
	if !(syncCheckOptions.Body == "" && syncCheckOptions.Path == "") {
		if err = base.CacheNodes(nodes.SyncCheckedNodes, options.Ac.CacheBatch, options.SyncCheckKey, options.CacheTTL); err != nil {
			logger.Log.WithFields(log.Fields{
				"error":        err.Error(),
				"requestID":    options.Ac.RequestID,
				"blockchainID": options.Blockchain.ID,
				"sessionKey":   options.Session.Key,
			}).Error("perform checks: error caching sync check nodes: " + err.Error())
		}
		base.EraseNodesFailureMark(nodes.SyncCheckedNodes, options.Blockchain.ID, options.Ac.CommitHash, options.Ac.CacheBatch)
	}
}

func invokeChecks(options *base.PerformChecksOptions) (*performAppCheck.Response, error) {
	request := performAppCheck.Payload{
		Session:                *options.Session,
		Blockchain:             options.Blockchain,
		AAT:                    *options.PocketAAT,
		DefaultAllowance:       options.Ac.SyncChecker.DefaultSyncAllowance,
		AltruistTrustThreshold: options.Ac.SyncChecker.AltruistTrustThreshold,
		RequestID:              options.Ac.RequestID,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": options.Ac.RequestID,
			"error":     err.Error(),
		}).Error("perform checks: error marshalling payload: " + err.Error())
		return nil, err
	}

	result, err := client.Invoke(&awsLambda.InvokeInput{
		FunctionName: aws.String(performCheckFunctionName), Payload: payload})
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID":  options.Ac.RequestID,
			"error":      err.Error(),
			"statusCode": result.StatusCode,
		}).Error("perform checks: error invoking lambda: " + err.Error())
		return nil, err
	}

	var response events.APIGatewayProxyResponse
	if err = json.Unmarshal(result.Payload, &response); err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": options.Ac.RequestID,
			"error":     err.Error(),
		}).Error("perform checks: error unmarshalling invoke response")
		return nil, err
	}

	var validNodes performAppCheck.Response
	if err = json.Unmarshal([]byte(response.Body), &validNodes); err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": options.Ac.RequestID,
			"error":     err.Error(),
		}).Error("perform checks: error unmarshalling valid nodes: " + err.Error())
		return nil, err
	}

	return &validNodes, nil
}

func main() {
	lambda.Start(lambdaHandler)
}
