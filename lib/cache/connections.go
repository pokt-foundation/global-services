package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/errgroup"
)

// ConnectoCacheClients instantiates n number of cache connections and returns error
// if any of those connection attempts fails
func ConnectoCacheClients(connectionStrings []string, commitHash string) ([]*Redis, error) {
	clients := make(chan *Redis, len(connectionStrings))

	var wg sync.WaitGroup

	for _, address := range connectionStrings {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			err := connectToInstance(clients, addr, commitHash)
			if err != nil {
				// TODO: Add warn log
				fmt.Printf("failure connecting to redis instance %s: %s\n", addr, err.Error())
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

func connectToInstance(clients chan *Redis, address string, commitHash string) error {
	redisClient, err := NewRedisClusterClient(RedisClientOptions{
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

	clients <- redisClient

	return nil
}

func RunFunctionOnAllClients(cacheClients []*Redis, fn func(*Redis) error) error {
	var g errgroup.Group
	for _, cacheClient := range cacheClients {
		func(ch *Redis) {
			g.Go(func() error {
				return fn(ch)
			})
		}(cacheClient)
	}
	return g.Wait()
}
