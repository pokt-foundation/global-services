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
		}).Error("perform checks: error unmarshalling invoke response: " + err.Error())
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

// INVOKE RESPONSE: map[body:{"syncCheckNodes":["e18ac1f4530bcace336a707b7245d5ae7f04ec80c8af5f0a6bc1c407c9bba2a6","1dd5b62f464eb56f54b5371d4424794ba00a6b54eb1ee08d02dd911475ea0870","f19db312d8a7005a21f88d801ecd9eb4676c54ae8c4ee6cc18c67b23db4417aa","737ea6b225cd36ef7c7186c8678cd5e7ca503b617e12dc837cae0d54c537b7a9","459d89373a99f4b920ffed98a6246b2a102f85de1a5f1abc6a40ff74067dfec0","8e167b6c69b9f1272d3e84d33fcc9590fb31b8c37d6582c140d954df5c41cbe2","562fd17b59fcdc0a4e47fc785de063cdf89227e1af19ead545de6bc8f1d5a791","35f06b3f8c366058eea0f4b89ff82f06440b4e6cccd53d9b259311d198c2728b","341bf39b41eed530fc0bccad42fa9c680355ab5cb92960f9d372f0f27517ff22","eab4392e45827065ce541d0b8b28d10c76ddee5d12051b09c4fdd553ab90553e","1b2b0afeae26061b5b94a86d1eb68e9557cd11b99bb998bc91e2202ef37fcfcc","4aed73d443f85a40e7ad53c336001cae1048684bfbbe24c82903f55d5845987e","b203ed3a91e65cf58a9d8f35cd48a0f5f5f75ae2264dff1e02a65e6d41b518f3","e5099c17f0ce5b762c0cccb7266157212d4e7496899183e6965c632035b7f428","c6844281a9757dad070f2ed1e386f956274fdf4f16eab5d69b275b408b6bb2df","641f21fc31a06b24dcf6204242097adb985054e54ebf6136737174edfb6b9a99","f5c83abc216a1aa9c716cb1b6eacdbde80180cdcf2655f56c7705c0f48874717","1668c9a5079064516b18e1fc37ad76eb8ffb2b23e965e082ea7f7d020f3ccc4f","63104c7fc7a432ecf8b22c1e6fae43bdba0d6f658d2ca2027921144e957faac6","44cf8866bb64b316aadd928a853d8b20e54a428285eb7c5d05b3b2bbee4aa564"],"chainCheckNodes":["44cf8866bb64b316aadd928a853d8b20e54a428285eb7c5d05b3b2bbee4aa564","63104c7fc7a432ecf8b22c1e6fae43bdba0d6f658d2ca2027921144e957faac6","b203ed3a91e65cf58a9d8f35cd48a0f5f5f75ae2264dff1e02a65e6d41b518f3","f5c83abc216a1aa9c716cb1b6eacdbde80180cdcf2655f56c7705c0f48874717","c6844281a9757dad070f2ed1e386f956274fdf4f16eab5d69b275b408b6bb2df","341bf39b41eed530fc0bccad42fa9c680355ab5cb92960f9d372f0f27517ff22","1b2b0afeae26061b5b94a86d1eb68e9557cd11b99bb998bc91e2202ef37fcfcc","eab4392e45827065ce541d0b8b28d10c76ddee5d12051b09c4fdd553ab90553e","4aed73d443f85a40e7ad53c336001cae1048684bfbbe24c82903f55d5845987e","641f21fc31a06b24dcf6204242097adb985054e54ebf6136737174edfb6b9a99","1668c9a5079064516b18e1fc37ad76eb8ffb2b23e965e082ea7f7d020f3ccc4f","35f06b3f8c366058eea0f4b89ff82f06440b4e6cccd53d9b259311d198c2728b","459d89373a99f4b920ffed98a6246b2a102f85de1a5f1abc6a40ff74067dfec0","8e167b6c69b9f1272d3e84d33fcc9590fb31b8c37d6582c140d954df5c41cbe2","562fd17b59fcdc0a4e47fc785de063cdf89227e1af19ead545de6bc8f1d5a791","e18ac1f4530bcace336a707b7245d5ae7f04ec80c8af5f0a6bc1c407c9bba2a6","737ea6b225cd36ef7c7186c8678cd5e7ca503b617e12dc837cae0d54c537b7a9","e5099c17f0ce5b762c0cccb7266157212d4e7496899183e6965c632035b7f428","f19db312d8a7005a21f88d801ecd9eb4676c54ae8c4ee6cc18c67b23db4417aa","1dd5b62f464eb56f54b5371d4424794ba00a6b54eb1ee08d02dd911475ea0870"]} headers:map[Access-Control-Allow-Origin:* Content-Type:application/json] multiValueHeaders:<nil> statusCode:0]
