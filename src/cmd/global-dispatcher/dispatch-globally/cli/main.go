package main

import (
	"context"
	"time"

	base "github.com/Pocket/global-services/src/cmd/global-dispatcher/dispatch-globally"
	"github.com/Pocket/global-services/src/common/environment"

	logger "github.com/Pocket/global-services/src/services/logger"
	"github.com/Pocket/global-services/src/utils"
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