package main

import (
	"context"
	"fmt"
	"time"

	snapdata "github.com/Pocket/global-services/cherry-picker/cmd/snap-data"
	"github.com/Pocket/global-services/shared/environment"
	"github.com/Pocket/global-services/shared/utils"
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
		// TODO: LOG
		fmt.Println(err)
	}

	err = snapCherryPickerData.SnapCherryPickerData(ctx)
	if err != nil {
		// TODO: LOG
		fmt.Println(err)
	}

	fmt.Println("Done")
}
