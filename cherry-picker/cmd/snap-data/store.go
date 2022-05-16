package snapdata

import (
	"context"
	"fmt"
	"sync"

	cpicker "github.com/Pocket/global-services/cherry-picker"
	"github.com/Pocket/global-services/cherry-picker/database"
	logger "github.com/Pocket/global-services/shared/logger"
	"github.com/Pocket/global-services/shared/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

func (sn *SnapCherryPicker) saveToStore(ctx context.Context) {
	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(concurrency)

	sessionsInStore := map[string]*SessionKeys{}
	for _, region := range sn.Regions {
		for _, application := range region.AppData {
			wg.Add(1)
			sem.Acquire(ctx, 1)

			go func(rg *Region, app *ApplicationData) {
				defer wg.Done()
				defer sem.Release(1)

				sessionInStoreKey := fmt.Sprintf("%s-%s-%s", app.PublicKey, app.Chain, app.ServiceLog.SessionKey)
				if _, ok := sessionsInStore[sessionInStoreKey]; !ok {
					if err := sn.createSessionIfDoesntExist(ctx, app); err != nil {
						logger.Log.WithFields(log.Fields{
							"requestID":  sn.RequestID,
							"error":      err.Error(),
							"publicKey":  app.PublicKey,
							"chain":      app.Chain,
							"sessionKey": app.ServiceLog.SessionKey,
						}).Error("error creating session:", err.Error())
						return
					}
					sessionsInStore[sessionInStoreKey] = &SessionKeys{
						PublicKey:  app.PublicKey,
						Chain:      app.Chain,
						SessionKey: app.ServiceLog.SessionKey,
					}
				}

				if _, err := sn.createOrUpdateRegion(ctx, rg.Name, app); err != nil {
					logger.Log.WithFields(log.Fields{
						"requestID":  sn.RequestID,
						"error":      err.Error(),
						"publicKey":  app.PublicKey,
						"chain":      app.Chain,
						"sessionKey": app.ServiceLog.SessionKey,
					}).Error("error creating/updating region:", err.Error())
				}
			}(region, application)
		}
	}
	wg.Wait()

	sn.aggregateRegionData(ctx, sessionsInStore)
}

func (sn *SnapCherryPicker) aggregateRegionData(ctx context.Context, sessionsInStore map[string]*SessionKeys) {
	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(concurrency)

	for _, session := range sessionsInStore {
		wg.Add(1)
		sem.Acquire(ctx, 1)

		go func(sess *SessionKeys) {
			defer wg.Done()
			defer sem.Release(1)

			utils.RunFnOnSlice(sn.Stores, func(st cpicker.CherryPickerStore) error {
				regions, err := st.GetSessionRegions(ctx, sess.PublicKey, sess.Chain, sess.SessionKey)
				if err != nil {
					logger.Log.WithFields(log.Fields{
						"requestID":  sn.RequestID,
						"error":      err.Error(),
						"publicKey":  sess.PublicKey,
						"chain":      sess.Chain,
						"sessionKey": sess.SessionKey,
						"database":   st.GetConnection(),
					}).Error("error getting session regions:", err.Error())
					return err
				}
				session := &cpicker.UpdateSession{
					PublicKey:  sess.PublicKey,
					Chain:      sess.Chain,
					SessionKey: sess.SessionKey,
				}
				avgSuccessTimes := []float32{}

				for _, region := range regions {
					session.TotalSuccess += region.TotalSuccess
					session.TotalFailure += region.TotalFailure
					session.Failure = session.Failure || region.Failure
					avgSuccessTimes = append(avgSuccessTimes, region.MedianSuccessLatency...)
				}
				session.AverageSuccessTime = utils.AvgOfSlice(avgSuccessTimes)

				_, err = st.UpdateSession(ctx, session)
				if err != nil {
					logger.Log.WithFields(log.Fields{
						"requestID":  sn.RequestID,
						"error":      err.Error(),
						"publicKey":  sess.PublicKey,
						"chain":      sess.Chain,
						"sessionKey": sess.SessionKey,
						"database":   st.GetConnection(),
					}).Error("error updating session:", err.Error())
				}
				return err
			})
		}(session)
	}
}

func (sn *SnapCherryPicker) createOrUpdateRegion(ctx context.Context, regionName string,
	app *ApplicationData) (*cpicker.Region, error) {
	region := &cpicker.Region{
		PublicKey:                 app.PublicKey,
		Chain:                     app.Chain,
		SessionKey:                app.ServiceLog.SessionKey,
		Address:                   app.Address,
		Region:                    regionName,
		SessionHeight:             app.ServiceLog.SessionHeight,
		TotalSuccess:              app.Successes,
		TotalFailure:              app.Failures,
		MedianSuccessLatency:      []float32{app.MedianSuccessLatency},
		WeightedSuccessLatency:    []float32{app.WeightedSuccessLatency},
		AvgSuccessLatency:         app.MedianSuccessLatency,
		AvgWeightedSuccessLatency: app.WeightedSuccessLatency,
		Failure:                   app.Failure,
	}

	err := utils.RunFnOnSlice(sn.Stores, func(st cpicker.CherryPickerStore) error {
		storeRegion, err := st.GetRegion(ctx, app.PublicKey, app.Chain, app.ServiceLog.SessionKey, regionName)
		if err != nil {
			if err != database.ErrNotFound {
				return err
			}
			return st.CreateRegion(ctx, region)
		}

		updateRegion := &cpicker.UpdateRegion{
			PublicKey:              app.PublicKey,
			Chain:                  app.Chain,
			SessionKey:             app.ServiceLog.SessionKey,
			Region:                 regionName,
			TotalSuccess:           app.Successes,
			TotalFailure:           app.Failures,
			MedianSuccessLatency:   app.MedianSuccessLatency,
			WeightedSuccessLatency: app.WeightedSuccessLatency,
			AvgSuccessLatency: float32(utils.AvgOfSlice(
				append(storeRegion.MedianSuccessLatency, app.MedianSuccessLatency))),
			AvgWeightedSuccessLatency: float32(utils.AvgOfSlice(
				append(storeRegion.WeightedSuccessLatency, app.WeightedSuccessLatency))),
			Failure: app.Failure || storeRegion.Failure,
		}

		region, err = st.UpdateRegion(ctx, updateRegion)
		return err
	})

	return region, err
}

func (sn *SnapCherryPicker) createSessionIfDoesntExist(ctx context.Context, app *ApplicationData) error {
	return utils.RunFnOnSlice(sn.Stores, func(st cpicker.CherryPickerStore) error {
		_, err := st.GetSession(ctx, app.PublicKey, app.Chain, app.ServiceLog.SessionKey)
		if err == database.ErrNotFound {
			return st.CreateSession(ctx, &cpicker.Session{
				PublicKey:     app.PublicKey,
				Chain:         app.Chain,
				SessionKey:    app.ServiceLog.SessionKey,
				SessionHeight: app.ServiceLog.SessionHeight,
				Address:       app.Address,
			})
		}
		return err
	})
}
