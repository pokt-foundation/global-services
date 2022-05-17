package main

import (
	"context"
	"time"

	base "github.com/Pocket/global-services/global-dispatcher/cmd/dispatch"
	"github.com/Pocket/global-services/shared/environment"
	"github.com/Pocket/global-services/shared/utils"

	logger "github.com/Pocket/global-services/shared/logger"
	log "github.com/sirupsen/logrus"
)

var timeout = time.Duration(environment.GetInt64("TIMEOUT", 360)) * time.Second

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	requestID, _ := utils.RandomHex(32)
	failedDispatcherCalls, err := base.DispatchSessions(ctx, requestID)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID":      requestID,
			"error":          err.Error(),
			"failedDispatch": failedDispatcherCalls,
		}).Error("ERROR DISPATCHING SESSION: " + err.Error())
	}
	logger.Log.WithFields(log.Fields{
		"requestID":      requestID,
		"failedDispatch": failedDispatcherCalls,
	}).Info("GLOBAL DISPATCHER RESULT")
}
