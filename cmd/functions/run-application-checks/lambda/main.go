package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

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

type ApplicationData struct {
	payload *performAppCheck.Payload
	config  *base.PerformChecksOptions
	wg      *sync.WaitGroup
}

var (
	region                   = environment.GetString("AWS_REGION", "")
	performCheckFunctionName = environment.GetString("PERFORM_CHECK_FUNCTION_NAME", "")
	checksPerInvoke          = environment.GetInt64("CHECKS_PER_INVOKE", 10)

	sess = session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client = awsLambda.New(sess, &aws.Config{
		Region: aws.String(region)})
	application chan *ApplicationData
)

func lambdaHandler(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)
	requestID := lc.AwsRequestID
	// Prevent errors regarding the lambda caching the global variable value
	application = make(chan *ApplicationData, checksPerInvoke)

	go monitorAppBatch(ctx, requestID)

	err := base.RunApplicationChecks(ctx, requestID, getAppToCheck)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": lc.AwsRequestID,
			"error":     err.Error(),
		}).Error("ERROR RUNNING APPLICATION CHECKS: " + err.Error())
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(http.StatusOK, map[string]interface{}{
		"ok": true,
	}), err
}

func monitorAppBatch(ctx context.Context, requestID string) {
	batch := make(map[string]ApplicationData)
	processedApps := 0

	for {
		app := <-application
		if app != nil {
			if !app.config.Invalid {
				batch[app.config.Session.Header.AppPublicKey] = *app
			}
			processedApps++
		}

		processedAll := processedApps >= app.config.TotalApps
		if !processedAll && len(batch) < int(checksPerInvoke) {
			app.wg.Done()
			continue
		}

		go performChecks(batch, ctx, requestID, app.wg)
		batch = make(map[string]ApplicationData)
	}
}

func getAppToCheck(ctx context.Context, options *base.PerformChecksOptions) {
	var wg sync.WaitGroup
	wg.Add(1)
	application <- &ApplicationData{
		payload: &performAppCheck.Payload{
			Session:                *options.Session,
			Blockchain:             options.Blockchain,
			AAT:                    *options.PocketAAT,
			DefaultAllowance:       options.Ac.SyncChecker.DefaultSyncAllowance,
			AltruistTrustThreshold: options.Ac.SyncChecker.AltruistTrustThreshold,
			RequestID:              options.Ac.RequestID,
		},
		config: options,
		wg:     &wg,
	}
	wg.Wait()
}

func performChecks(apps map[string]ApplicationData, ctx aws.Context, requestID string, wg *sync.WaitGroup) {
	defer wg.Done()

	payloads := []*performAppCheck.Payload{}
	for _, app := range apps {
		payloads = append(payloads, app.payload)
	}

	nodeSet, err := invokeChecks(payloads, requestID)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": requestID,
			"error":     err.Error(),
		}).Error("perform checks: error invoking check: " + err.Error())
		return
	}

	for publicKey, nodes := range nodeSet.ChainCheckedNodes {
		options := apps[publicKey].config

		if options.Blockchain.ChainIDCheck != "" {
			if err = base.CacheNodes(nodes, options.Ac.CacheBatch, options.ChainCheckKey, options.CacheTTL); err != nil {
				logger.Log.WithFields(log.Fields{
					"error":        err.Error(),
					"requestID":    options.Ac.RequestID,
					"blockchainID": options.Blockchain.ID,
					"sessionKey":   options.Session.Key,
				}).Error("perform checks: error caching chain check nodes: " + err.Error())
			}
		}
	}

	for publicKey, nodes := range nodeSet.SyncCheckedNodes {
		options := apps[publicKey].config

		syncCheckOptions := options.Blockchain.SyncCheckOptions
		if !(syncCheckOptions.Body == "" && syncCheckOptions.Path == "") {
			if err = base.CacheNodes(nodes, options.Ac.CacheBatch, options.SyncCheckKey, options.CacheTTL); err != nil {
				logger.Log.WithFields(log.Fields{
					"error":        err.Error(),
					"requestID":    options.Ac.RequestID,
					"blockchainID": options.Blockchain.ID,
					"sessionKey":   options.Session.Key,
				}).Error("perform checks: error caching sync check nodes: " + err.Error())
			}
			base.EraseNodesFailureMark(nodes, options.Blockchain.ID, options.Ac.CommitHash, options.Ac.CacheBatch)
		}
	}
}

func invokeChecks(apps []*performAppCheck.Payload, requestID string) (*performAppCheck.Response, error) {
	payload, err := json.Marshal(apps)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": requestID,
			"error":     err.Error(),
		}).Error("perform checks: error marshalling payload: " + err.Error())
		return nil, err
	}

	result, err := client.Invoke(&awsLambda.InvokeInput{
		FunctionName: aws.String(performCheckFunctionName), Payload: payload})
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID":  requestID,
			"error":      err.Error(),
			"statusCode": result.StatusCode,
		}).Error("perform checks: error invoking lambda: " + err.Error())
		return nil, err
	}

	var response events.APIGatewayProxyResponse
	if err = json.Unmarshal(result.Payload, &response); err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": requestID,
			"error":     err.Error(),
		}).Error("perform checks: error unmarshalling invoke response")
		return nil, err
	}

	var validNodes *performAppCheck.Response
	if err = json.Unmarshal([]byte(response.Body), &validNodes); err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": requestID,
			"error":     err.Error(),
		}).Error("perform checks: error unmarshalling valid nodes: " + err.Error())
		return nil, err
	}

	return validNodes, nil
}

func main() {
	lambda.Start(lambdaHandler)
}
