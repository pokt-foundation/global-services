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

	sessionsInStore := sync.Map{}
	for _, region := range sn.Regions {
		for _, application := range region.AppData {
			if !validateAppData(application) {
				continue
			}

			wg.Add(1)
			sem.Acquire(ctx, 1)

			go func(rg *Region, app *CherryPickerData) {
				defer wg.Done()
				defer sem.Release(1)

				sessionInStoreKey := fmt.Sprintf("%s-%s-%s", app.PublicKey, app.Chain, app.ServiceLog.SessionKey)
				if _, ok := sessionsInStore.Load(sessionInStoreKey); !ok {
					errs := sn.createSessionIfDoesntExist(ctx, app)
					for idx, err := range errs {
						if err != nil {
							logger.Log.WithFields(log.Fields{
								"requestID":  sn.RequestID,
								"error":      err.Error(),
								"publicKey":  app.PublicKey,
								"chain":      app.Chain,
								"sessionKey": app.ServiceLog.SessionKey,
								"database":   sn.Stores[idx].GetConnection(),
							}).Error("error creating session:", err.Error())
							return
						}
					}
					sessionsInStore.Store(sessionInStoreKey, &SessionKeys{
						PublicKey:  app.PublicKey,
						Chain:      app.Chain,
						SessionKey: app.ServiceLog.SessionKey,
					})
				}

				_, errs := sn.createOrUpdateRegion(ctx, rg.Name, app)
				for idx, err := range errs {
					if err != nil {
						logger.Log.WithFields(log.Fields{
							"requestID":  sn.RequestID,
							"error":      err.Error(),
							"publicKey":  app.PublicKey,
							"chain":      app.Chain,
							"sessionKey": app.ServiceLog.SessionKey,
							"database":   sn.Stores[idx].GetConnection(),
						}).Error("error creating/updating region:", err.Error())
					}
				}

			}(region, application)
		}
	}
	wg.Wait()

	sessInStore := map[string]*SessionKeys{}
	sessionsInStore.Range(func(key, value any) bool {
		sessInStore[key.(string)] = value.(*SessionKeys)
		return true
	})
	sn.aggregateRegionData(ctx, sessInStore)
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

			utils.RunFnOnSliceMultipleFailures(sn.Stores, func(st cpicker.CherryPickerStore) error {
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
				session := &cpicker.SessionUpdatePayload{
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
				session.AverageSuccessTime = float32(utils.GetSliceAvg(avgSuccessTimes))

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

	wg.Wait()
}

func (sn *SnapCherryPicker) createOrUpdateRegion(ctx context.Context, regionName string,
	app *CherryPickerData) (*cpicker.Region, []error) {
	region := &cpicker.Region{
		PublicKey:                 app.PublicKey,
		Chain:                     app.Chain,
		SessionKey:                app.ServiceLog.SessionKey,
		Address:                   app.Address,
		ApplicationPublicKey:      app.ServiceLog.Metadata.ApplicationPublicKey,
		Region:                    regionName,
		SessionHeight:             app.ServiceLog.SessionHeight,
		TotalSuccess:              app.Successes,
		TotalFailure:              app.Failures,
		MedianSuccessLatency:      []float32{app.MedianSuccessLatency},
		WeightedSuccessLatency:    []float32{app.WeightedSuccessLatency},
		AvgSuccessLatency:         app.MedianSuccessLatency,
		AvgWeightedSuccessLatency: app.WeightedSuccessLatency,
		P90Latency:                []float32{app.ServiceLog.Metadata.P90},
		Attempts:                  []int{app.ServiceLog.Metadata.Attempts},
		SuccessRate:               []float32{app.ServiceLog.Metadata.SuccessRate},
		Failure:                   app.Failure,
	}

	err := utils.RunFnOnSliceMultipleFailures(sn.Stores, func(st cpicker.CherryPickerStore) error {
		storeRegion, err := st.GetRegion(ctx, app.PublicKey, app.Chain, app.ServiceLog.SessionKey, regionName)
		if err != nil {
			if err != database.ErrNotFound {
				return err
			}
			return st.CreateRegion(ctx, region)
		}

		updateRegion := &cpicker.RegionUpdatePayload{
			PublicKey:              app.PublicKey,
			Chain:                  app.Chain,
			SessionKey:             app.ServiceLog.SessionKey,
			Region:                 regionName,
			TotalSuccess:           app.Successes,
			TotalFailure:           app.Failures,
			MedianSuccessLatency:   app.MedianSuccessLatency,
			WeightedSuccessLatency: app.WeightedSuccessLatency,
			AvgSuccessLatency: float32(utils.GetSliceAvg(
				append(storeRegion.MedianSuccessLatency, app.MedianSuccessLatency))),
			AvgWeightedSuccessLatency: float32(utils.GetSliceAvg(
				append(storeRegion.WeightedSuccessLatency, app.WeightedSuccessLatency))),
			P90Latency:  app.ServiceLog.Metadata.P90,
			Attempts:    app.ServiceLog.Metadata.Attempts,
			SuccessRate: app.ServiceLog.Metadata.SuccessRate,
			Failure:     app.Failure || storeRegion.Failure,
		}

		region, err = st.UpdateRegion(ctx, updateRegion)
		return err
	})

	return region, err
}

func (sn *SnapCherryPicker) createSessionIfDoesntExist(ctx context.Context, app *CherryPickerData) []error {
	return utils.RunFnOnSliceMultipleFailures(sn.Stores, func(st cpicker.CherryPickerStore) error {
		_, err := st.GetSession(ctx, app.PublicKey, app.Chain, app.ServiceLog.SessionKey)
		if err == database.ErrNotFound {
			return st.CreateSession(ctx, &cpicker.Session{
				PublicKey:            app.PublicKey,
				Chain:                app.Chain,
				SessionKey:           app.ServiceLog.SessionKey,
				SessionHeight:        app.ServiceLog.SessionHeight,
				Address:              app.Address,
				ApplicationPublicKey: app.ServiceLog.Metadata.ApplicationPublicKey,
			})
		}
		return err
	})
}

func validateAppData(app *CherryPickerData) bool {
	if app.PublicKey == "" || app.ServiceLog.SessionKey == "" {
		return false
	}
	if len(app.PublicKey) != 64 {
		return false
	}
	return true
}
