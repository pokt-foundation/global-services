package snapdata

import (
	"context"
	"strconv"
	"strings"

	"github.com/Pocket/global-services/shared/cache"
	logger "github.com/Pocket/global-services/shared/logger"
	"github.com/Pocket/global-services/shared/utils"
	poktutils "github.com/pokt-foundation/pocket-go/utils"
	log "github.com/sirupsen/logrus"
)

func (sn *SnapCherryPicker) getAppsRegionsData(ctx context.Context) error {
	return utils.RunFnOnSlice(sn.Caches, func(cl *cache.Redis) error {
		if err := sn.getServiceLogData(ctx, cl); err != nil {
			return err
		}
		return sn.getSuccessAndFailureData(ctx, cl)
	})
}

func (sn *SnapCherryPicker) getServiceLogData(ctx context.Context, cl *cache.Redis) error {
	serviceLogKeys, err := cl.Client.Keys(ctx, "*service*").Result()
	if err != nil {
		return err
	}

	if len(serviceLogKeys) == 0 {
		logger.Log.WithFields(log.Fields{
			"requestID": sn.RequestID,
			"region":    cl.Name,
		}).Warnf("no service log keys found for:", cl.Name)
		return nil
	}

	results, err := cl.MGetPipe(ctx, serviceLogKeys)
	if err != nil {
		return err
	}

	for idx, rawServiceLog := range results {
		publicKey, chain := getPublicKeyAndChainFromLog(serviceLogKeys[idx])
		appDataKey := publicKey + "-" + chain
		sn.Regions[cl.Name].AppData[appDataKey] = &ApplicationData{}
		appData := sn.Regions[cl.Name].AppData[appDataKey]
		appData.PublicKey = publicKey
		appData.Chain = chain
		address, _ := poktutils.GetAddressFromPublickey(publicKey)
		appData.Address = address

		if err := cache.UnMarshallJSONResult(rawServiceLog, nil, &appData.ServiceLog); err != nil {
			logger.Log.WithFields(log.Fields{
				"requestID":     sn.RequestID,
				"error":         err.Error(),
				"publicKey":     publicKey,
				"chain":         chain,
				"region":        cl.Name,
				"rawServiceLog": rawServiceLog,
			}).Error("error unmarshalling service log:", err.Error())
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
		return err
	}
	failuresKeys, err := cl.Client.Keys(ctx, "*"+failuresKey).Result()
	if err != nil {
		return err
	}
	failureKeys, err := cl.Client.Keys(ctx, "*"+failureKey).Result()
	if err != nil {
		return err
	}

	allKeys := append(successKeys, failuresKeys...)
	allKeys = append(allKeys, failureKeys...)
	results, err := cl.MGetPipe(ctx, allKeys)
	if err != nil {
		return err
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
