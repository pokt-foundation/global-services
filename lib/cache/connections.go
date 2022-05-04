package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Pocket/global-services/lib/logger"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// ConnectoCacheClients instantiates n number of cache connections and returns error
// if any of those connection attempts fails
func ConnectoCacheClients(ctx context.Context, connectionStrings []string, commitHash string, isCluster bool) ([]*Redis, error) {
	clients := make(chan *Redis, len(connectionStrings))

	var wg sync.WaitGroup

	for _, address := range connectionStrings {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			err := connectToInstance(ctx, clients, addr, commitHash, isCluster)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{
					"address": addr,
					"error":   err.Error(),
				}).Warn(fmt.Sprintf("failure connecting to redis instance %s: %s\n", addr, err.Error()))
			}
		}(address)
	}

	wg.Wait()

	close(clients)

	var instances []*Redis
	for client := range clients {
		instances = append(instances, client)
	}

	if len(instances) == 0 {
		return nil, errors.New("redis connection error: all instances failed to connect")
	}

	return instances, nil
}

// WriteJSONToCaches writes the given key/values to multiple cache clients at the same time
func WriteJSONToCaches(ctx context.Context, cacheClients []*Redis, key string, value interface{}, TTLSeconds uint) error {
	return RunFunctionOnAllClients(cacheClients, func(ins *Redis) error {
		return ins.SetJSON(ctx, key, value, TTLSeconds)
	})
}

// CloseConnections closes all cache connections, returning error if any of them fail
func CloseConnections(cacheClients []*Redis) error {
	return RunFunctionOnAllClients(cacheClients, func(ins *Redis) error {
		return ins.Close()
	})
}

func connectToInstance(ctx context.Context, clients chan *Redis, address string, commitHash string, isCluster bool) error {
	var redisClient *Redis
	var err error

	if isCluster {
		redisClient, err = NewRedisClusterClient(ctx, &RedisClientOptions{
			BaseOptions: &redis.Options{
				Addr:     address,
				Password: "",
				DB:       0,
			},
			KeyPrefix: commitHash,
		})
		if err != nil {
			return err
		}
	} else {
		redisClient, err = NewRedisClient(ctx, &RedisClientOptions{
			BaseOptions: &redis.Options{
				Addr:     address,
				Password: "",
				DB:       0,
			},
			KeyPrefix: commitHash,
		})
		if err != nil {
			return err
		}
	}

	clients <- redisClient

	return nil
}

func RunFunctionOnAllClients(caches []*Redis, fn func(*Redis) error) error {
	var g errgroup.Group
	for _, cacheClient := range caches {
		func(ch *Redis) {
			g.Go(func() error {
				return fn(ch)
			})
		}(cacheClient)
	}
	return g.Wait()
}
