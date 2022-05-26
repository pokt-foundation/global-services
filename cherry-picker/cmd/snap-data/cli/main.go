package main

import (
	"context"
	"fmt"
	"os"
	"time"

	snapdata "github.com/Pocket/global-services/cherry-picker/cmd/snap-data"
	postgresdb "github.com/Pocket/global-services/cherry-picker/database"
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
	defer clean(snapCherryPickerData)

	err := snapCherryPickerData.Init(ctx)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": snapCherryPickerData.RequestID,
			"error":     err.Error(),
		}).Error("error initializing:", err.Error())
		fmt.Println(err)
		os.Exit(1)
	}

	snapCherryPickerData.SnapCherryPickerData(ctx)

	fmt.Println("Done")
}

func clean(sn *snapdata.SnapCherryPicker) {
	for _, store := range sn.Stores {
		postgres, ok := store.(*postgresdb.CherryPickerPostgres)
		if !ok {
			continue
		}
		postgres.Db.Conn.Close()
	}

	for _, cache := range sn.Caches {
		cache.Close()
	}
}
