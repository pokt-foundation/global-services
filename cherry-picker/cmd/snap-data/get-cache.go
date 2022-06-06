package snapdata

import (
	"context"
	"strconv"
	"strings"

	"github.com/Pocket/global-services/shared/cache"
	logger "github.com/Pocket/global-services/shared/logger"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/pkg/errors"
	poktutils "github.com/pokt-foundation/pocket-go/utils"
	log "github.com/sirupsen/logrus"
)

func (sn *SnapCherryPicker) getAppsRegionsData(ctx context.Context) {
	errs := utils.RunFnOnSliceMultipleFailures(sn.Caches, func(cl *cache.Redis) error {
		if err := sn.getServiceLogData(ctx, cl); err != nil {
			return err
		}
		return sn.getSuccessAndFailureData(ctx, cl)
	})
	for idx, err := range errs {
		if err != nil {
			logger.Log.WithFields(log.Fields{
				"requestID": sn.RequestID,
				"error":     err.Error(),
				"region":    sn.Caches[idx].Name,
			}).Error("error getting cherry picker data")
		}
	}
}

func (sn *SnapCherryPicker) getServiceLogData(ctx context.Context, cl *cache.Redis) error {
	serviceLogKeys, err := cl.Client.Keys(ctx, "*service*").Result()
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": sn.RequestID,
			"error":     err.Error(),
			"region":    cl.Name,
		}).Error("error getting service log keys")
		return err
	}

	if len(serviceLogKeys) == 0 {
		logger.Log.WithFields(log.Fields{
			"requestID": sn.RequestID,
			"region":    cl.Name,
		}).Warn("no service log keys found for:", cl.Name)
		return nil
	}

	results, err := cl.MGetPipe(ctx, serviceLogKeys)
	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID": sn.RequestID,
			"error":     err.Error(),
			"region":    cl.Name,
		}).Error("error getting service log values")
		return err
	}

	for idx, rawServiceLog := range results {
		publicKey, chain := getPublicKeyAndChainFromLog(serviceLogKeys[idx])
		appDataKey := publicKey + "-" + chain
		sn.Regions[cl.Name].AppData[appDataKey] = &CherryPickerData{}
		appData := sn.Regions[cl.Name].AppData[appDataKey]
		appData.PublicKey = publicKey
		appData.Chain = chain
		address, _ := poktutils.GetAddressFromPublickey(publicKey)
		appData.Address = address

		if err := cache.UnmarshallJSONResult(rawServiceLog, nil, &appData.ServiceLog); err != nil {
			logger.Log.WithFields(log.Fields{
				"requestID":     sn.RequestID,
				"error":         err.Error(),
				"publicKey":     publicKey,
				"chain":         chain,
				"region":        cl.Name,
				"rawServiceLog": rawServiceLog,
			}).Error("error unmarshalling service log: ", err.Error())
			continue
		}

		medianSuccessLatency, err := strconv.ParseFloat(appData.ServiceLog.MedianSuccessLatency, 32)
		if err != nil {
			logger.Log.WithFields(log.Fields{
				"requestID": sn.RequestID,
				"error":     err.Error(),
				"publicKey": publicKey,
				"chain":     chain,
				"region":    cl.Name,
			}).Error("error casting median success latency:", err.Error())
			continue
		}

		appData.MedianSuccessLatency = float32(medianSuccessLatency)

		weightedSuccessLatency, err := strconv.ParseFloat(appData.ServiceLog.WeightedSuccessLatency, 32)
		if err != nil {
			logger.Log.WithFields(log.Fields{
				"requestID": sn.RequestID,
				"error":     err.Error(),
				"publicKey": publicKey,
				"chain":     chain,
				"region":    cl.Name,
			}).Error("error casting weigthed success latency:", err.Error())
		}

		appData.WeightedSuccessLatency = float32(weightedSuccessLatency)
	}

	return nil
}

func (sn *SnapCherryPicker) getSuccessAndFailureData(ctx context.Context, cl *cache.Redis) error {
	successKeys, err := cl.Client.Keys(ctx, "*"+successKey).Result()
	if err != nil {
		return errors.Wrap(err, "err getting success keys")
	}
	failuresKeys, err := cl.Client.Keys(ctx, "*"+failuresKey).Result()
	if err != nil {
		return errors.Wrap(err, "err getting failures keys")
	}
	failureKeys, err := cl.Client.Keys(ctx, "*"+failureKey).Result()
	if err != nil {
		return errors.Wrap(err, "err getting failure keys")
	}

	allKeys := append(successKeys, failuresKeys...)
	allKeys = append(allKeys, failureKeys...)
	results, err := cl.MGetPipe(ctx, allKeys)
	if err != nil {
		return errors.Wrap(err, "err getting success/failure values")
	}

	for idx, rawResult := range results {
		key := allKeys[idx]
		publicKey, chain := getPublicKeyAndChainFromLog(key)
		appDataKey := publicKey + "-" + chain
		region := sn.Regions[cl.Name].AppData[appDataKey]
		result, _ := cache.GetStringResult(rawResult, nil)
		if region == nil {
			continue
		}

		switch {
		case strings.Contains(key, successKey):
			sessionKey := strings.Split(key, "-")[2]
			successes, err := strconv.Atoi(result)
			if err != nil || region.ServiceLog.SessionKey != sessionKey {
				continue
			}
			region.Successes = successes
		case strings.Contains(key, failuresKey):
			sessionKey := strings.Split(key, "-")[2]
			failures, err := strconv.Atoi(result)
			if err != nil || region.ServiceLog.SessionKey != sessionKey {
				continue
			}
			region.Failures = failures
		case strings.Contains(key, failureKey):
			region.Failure = result == "true"
		}
	}

	return nil
}

func getPublicKeyAndChainFromLog(key string) (string, string) {
	split := strings.Split(key, "-")

	publicKey := split[1]
	chain := split[0]
	// Chain key could have braces between to use the same slot on a redis cluster
	// https://redis.com/blog/redis-clustering-best-practices-with-keys/
	chain = strings.ReplaceAll(chain, "{", "")
	chain = strings.ReplaceAll(chain, "}", "")
	return publicKey, chain
}
