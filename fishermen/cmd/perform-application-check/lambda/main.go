package main

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Pocket/global-services/shared/apigateway"
	"github.com/Pocket/global-services/shared/database"
	"github.com/Pocket/global-services/shared/environment"
	"github.com/Pocket/global-services/shared/metrics"
	"github.com/Pocket/global-services/shared/pocket"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pokt-foundation/pocket-go/provider"
	"github.com/pokt-foundation/pocket-go/relayer"
	"github.com/pokt-foundation/pocket-go/signer"

	models "github.com/Pocket/global-services/fishermen/cmd/perform-application-check"
	logger "github.com/Pocket/global-services/shared/logger"
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
	minMetricsPoolSize = 2
	maxMetricsPoolSize = 2
)

func lambdaHandler(ctx context.Context, payload []models.Payload) (events.APIGatewayProxyResponse, error) {
	syncChecks, chainChecks, mergeChecks, err := performApplicationChecks(ctx, payload, payload[0].RequestID)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"error":     err.Error(),
			"requestID": payload[0].RequestID,
		}).Errorf("perform application check error: %s", err.Error())
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(http.StatusOK, models.Response{
		SyncCheckedNodes:  syncChecks,
		ChainCheckedNodes: chainChecks,
		MergeCheckNodes:   mergeChecks,
	}), err
}

func performApplicationChecks(ctx context.Context, payload []models.Payload, requestID string) (
	syncChecks map[string][]string, chainChecks map[string][]string, mergeChecks map[string][]string, err error) {
	syncChecks = make(map[string][]string)
	chainChecks = make(map[string][]string)
	mergeChecks = make(map[string][]string)

	metricsRecorder, err := metrics.NewMetricsRecorder(ctx, &database.PostgresOptions{
		Connection:  metricsConnection,
		MinPoolSize: minMetricsPoolSize,
		MaxPoolSize: maxMetricsPoolSize,
	})
	if err != nil {
		return
	}

	rpcProvider := provider.NewProvider(rpcURL, dispatchURLs)
	rpcProvider.UpdateRequestConfig(0, defaultTimeOut)
	signer, err := signer.NewSignerFromPrivateKey(appPrivateKey)
	if err != nil {
		return
	}
	relayer := relayer.NewRelayer(signer, rpcProvider)

	var wg sync.WaitGroup
	for index, application := range payload {
		wg.Add(1)
		go func(idx int, app models.Payload) {
			defer wg.Done()
			syncCheck, chainCheck, mergeCheck := doPerformApplicationChecks(ctx, &app, metricsRecorder, relayer, requestID)
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
			mergeChecks[app.Session.Header.AppPublicKey] = mergeCheck
		}(index, application)
	}
	wg.Wait()

	return
}

func doPerformApplicationChecks(ctx context.Context, payload *models.Payload, metricsRecorder *metrics.Recorder, pocketRelayer *relayer.Relayer, requestID string) ([]string, []string, []string) {
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

	wg.Add(1)
	mergedNodes := []string{}
	go func() {
		defer wg.Done()
		if payload.Blockchain.ChainID != "0021" {
			return
		}

		mergeChecker := &pocket.MergeChecker{
			Relayer:         pocketRelayer,
			MetricsRecorder: metricsRecorder,
			RequestID:       requestID,
		}
		mergedNodes = mergeChecker.Check(ctx, pocket.MergeCheckOptions{
			Session:    payload.Session,
			PocketAAT:  payload.AAT,
			Blockchain: payload.Blockchain.ID,
			ChainID:    payload.Blockchain.ChainID,
			Path:       payload.Blockchain.Path,
		})
	}()

	return syncCheckNodes, chainCheckNodes, mergedNodes
}

func main() {
	lambda.Start(lambdaHandler)
}
