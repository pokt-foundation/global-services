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
	"github.com/pokt-foundation/pocket-go/pkg/provider"
	"github.com/pokt-foundation/pocket-go/pkg/relayer"
	"github.com/pokt-foundation/pocket-go/pkg/signer"

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

	metricsRecorder *metrics.Recorder
)

const (
	MIN_METRICS_POOL_SIZE = 2
	MAX_METRICS_POOL_SIZE = 2
)

func lambdaHandler(ctx context.Context, payload base.Payload) (events.APIGatewayProxyResponse, error) {
	var statusCode int

	syncCheckNodes, chainCheckNodes, err := PerformApplicationCheck(ctx, &payload, payload.RequestID)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"error":        err.Error(),
			"requestID":    payload.RequestID,
			"blockchainID": payload.Blockchain.ID,
			"sessionKey":   payload.Session.Key,
		}).Errorf("perform application check error: %s", err.Error())
		return *apigateway.NewErrorResponse(http.StatusInternalServerError, err), err
	}

	return *apigateway.NewJSONResponse(statusCode, base.Response{
		SyncCheckedNodes:  syncCheckNodes,
		ChainCheckedNodes: chainCheckNodes,
	}), err
}

func PerformApplicationCheck(ctx context.Context, payload *base.Payload, requestID string) ([]string, []string, error) {
	var err error
	metricsRecorder, err = metrics.NewMetricsRecorder(ctx, metricsConnection, MIN_METRICS_POOL_SIZE, MAX_METRICS_POOL_SIZE)
	if err != nil {
		return nil, nil, errors.New("error connecting to metrics db: " + err.Error())
	}

	rpcProvider := provider.NewJSONRPCProvider(rpcURL, dispatchURLs)
	rpcProvider.UpdateRequestConfig(0, defaultTimeOut)
	wallet, err := signer.NewWalletFromPrivatekey(appPrivateKey)
	if err != nil {
		return nil, nil, errors.New("error creating wallet: " + err.Error())
	}
	pocketRelayer := relayer.NewPocketRelayer(wallet, rpcProvider)

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
