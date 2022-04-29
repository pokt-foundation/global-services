package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Pocket/global-dispatcher/common/apigateway"
	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/Pocket/global-dispatcher/lib/metrics"
	"github.com/Pocket/global-dispatcher/lib/pocket"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pokt-foundation/pocket-go/provider"
	"github.com/pokt-foundation/pocket-go/relayer"
	"github.com/pokt-foundation/pocket-go/signer"

	base "github.com/Pocket/global-dispatcher/cmd/functions/perform-application-check"
	logger "github.com/Pocket/global-dispatcher/lib/logger"
	log "github.com/sirupsen/logrus"
)

var (
	rpcURL            = environment.GetString("RPC_URL", "")
	dispatchURLs      = strings.Split(environment.GetString("DISPATCH_URLS", ""), ",")
	appPrivateKey     = environment.GetString("APPLICATION_PRIVATE_KEY", "")
	defaultTimeOut    = time.Duration(environment.GetInt64("DEFAULT_TIMEOUT", 8)) * time.Second
	metricsConnection = environment.GetString("METRICS_CONNECTION", "")
)

const (
	MIN_METRICS_POOL_SIZE = 2
	MAX_METRICS_POOL_SIZE = 2
)

func lambdaHandler(ctx context.Context, payload []base.Payload) (events.APIGatewayProxyResponse, error) {
	syncChecks, chainChecks, err := PerformApplicationChecks(ctx, payload, payload[0].RequestID)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"error":     err.Error(),
			"requestID": payload[0].RequestID,
		}).Errorf("perform application check error: %s", err.Error())
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(http.StatusOK, base.Response{
		SyncCheckedNodes:  syncChecks,
		ChainCheckedNodes: chainChecks,
	}), err
}

func PerformApplicationChecks(ctx context.Context, payload []base.Payload, requestID string) (map[string][]string, map[string][]string, error) {
	metricsRecorder, err := metrics.NewMetricsRecorder(ctx, metricsConnection, MIN_METRICS_POOL_SIZE, MAX_METRICS_POOL_SIZE)
	if err != nil {
		return nil, nil, errors.New("error connecting to metrics db: " + err.Error())
	}

	rpcProvider := provider.NewProvider(rpcURL, dispatchURLs)
	rpcProvider.UpdateRequestConfig(0, defaultTimeOut)
	signer, err := signer.NewSignerFromPrivateKey(appPrivateKey)
	if err != nil {
		return nil, nil, errors.New("error creating signer: " + err.Error())
	}
	relayer := relayer.NewRelayer(signer, rpcProvider)

	syncChecks := map[string][]string{}
	chainChecks := map[string][]string{}

	var wg sync.WaitGroup
	for index, application := range payload {
		wg.Add(1)
		go func(idx int, app base.Payload) {
			defer wg.Done()
			syncCheck, chainCheck, err := doPerformApplicationChecks(ctx, &app, metricsRecorder, relayer, requestID)
			if err != nil {
				logger.Log.WithFields(log.Fields{
					"error":        err.Error(),
					"requestID":    app.RequestID,
					"blockchainID": app.Blockchain.ID,
					"sessionKey":   app.Session.Key,
				}).Errorf("perform application check error: %s", err.Error())
			}
			syncChecks[app.Session.Header.AppPublicKey] = syncCheck
			chainChecks[app.Session.Header.AppPublicKey] = chainCheck

		}(index, application)
	}
	wg.Wait()

	return syncChecks, chainChecks, nil
}

func doPerformApplicationChecks(ctx context.Context, payload *base.Payload, metricsRecorder *metrics.Recorder, pocketRelayer *relayer.Relayer, requestID string) ([]string, []string, error) {
	var wg sync.WaitGroup
	wg.Add(1)
	syncCheckNodes := []string{}
	go func() {
		defer wg.Done()

		syncCheckOptions := payload.Blockchain.SyncCheckOptions
		if syncCheckOptions.Body == "" && syncCheckOptions.Path == "" {
			return
		}

		syncChecker := &pocket.SyncChecker{
			Relayer:                pocketRelayer,
			DefaultSyncAllowance:   payload.DefaultAllowance,
			AltruistTrustThreshold: payload.AltruistTrustThreshold,
			MetricsRecorder:        metricsRecorder,
			RequestID:              requestID,
		}
		syncCheckNodes = syncChecker.Check(ctx, pocket.SyncCheckOptions{
			Session:          payload.Session,
			PocketAAT:        payload.AAT,
			SyncCheckOptions: syncCheckOptions,
			AltruistURL:      payload.Blockchain.Altruist,
			Blockchain:       payload.Blockchain.ID,
		})
	}()

	chainCheckNodes := []string{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if payload.Blockchain.ChainIDCheck == "" {
			return
		}

		chainChecker := &pocket.ChainChecker{
			Relayer:         pocketRelayer,
			MetricsRecorder: metricsRecorder,
			RequestID:       requestID,
		}
		chainCheckNodes = chainChecker.Check(ctx, pocket.ChainCheckOptions{
			Session:    payload.Session,
			PocketAAT:  payload.AAT,
			Blockchain: payload.Blockchain.ID,
			Data:       payload.Blockchain.ChainIDCheck,
			ChainID:    payload.Blockchain.ChainID,
			Path:       payload.Blockchain.Path,
		})
	}()

	wg.Wait()

	return syncCheckNodes, chainCheckNodes, nil
}

func main() {
	lambda.Start(lambdaHandler)
}
