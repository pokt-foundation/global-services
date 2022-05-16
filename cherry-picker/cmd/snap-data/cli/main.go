package main

import (
	"context"
	"fmt"
	"time"

	snapdata "github.com/Pocket/global-services/cherry-picker/cmd/snap-data"
	"github.com/Pocket/global-services/shared/environment"
	logger "github.com/Pocket/global-services/shared/logger"
	"github.com/Pocket/global-services/shared/utils"
	log "github.com/sirupsen/logrus"
)

var timeout = time.Duration(environment.GetInt64("TIMEOUT", 360)) * time.Second

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	requestID, _ := utils.RandomHex(32)

	snapCherryPickerData := &snapdata.SnapCherryPicker{
		RequestID: requestID,
	}
	snapCherryPickerData.Regions = make(map[string]*snapdata.Region)

	err := snapCherryPickerData.Init(ctx)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": snapCherryPickerData.RequestID,
			"error":     err.Error(),
		}).Error("error initializing:", err.Error())
		fmt.Println(err)
	}

	err = snapCherryPickerData.SnapCherryPickerData(ctx)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": snapCherryPickerData.RequestID,
			"error":     err.Error(),
		}).Error("error getting cherry picker data:", err.Error())
		fmt.Println(err)
	}

	fmt.Println("Done")
}
