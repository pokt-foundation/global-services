package cache

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/errgroup"
)

// GetCacheClients instantiates n number of cache connections and returns error
// if any of those connection attempts fails
func GetCacheClients(connectionStrings []string, commitHash string) ([]*Redis, error) {
	clients := make(chan *Redis, len(connectionStrings))

	var g errgroup.Group

	for _, address := range connectionStrings {
		func(addr string) {
			g.Go(func() error {
				return connectToInstance(clients, addr, commitHash)
			})
		}(address)
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	close(clients)

	var instances []*Redis
	for client := range clients {
		instances = append(instances, client)
	}

	return instances, nil
}

// WriteJSONToCaches writes the given key/values to multiple cache clients at the same time
func WriteJSONToCaches(cacheClients []*Redis, key string, value interface{}, TTLSeconds uint) error {
	var g errgroup.Group
	for _, cacheClient := range cacheClients {
		func(ch *Redis) {
			g.Go(func() error {
				ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
				defer cancel()

				return ch.SetJSON(ctx, key, value, TTLSeconds)
			})
		}(cacheClient)
	}

	return g.Wait()
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
