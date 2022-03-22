package cache

import (
	"context"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/errgroup"
)

// ConnectoCacheClients instantiates n number of cache connections and returns error
// if any of those connection attempts fails
func ConnectoCacheClients(connectionStrings []string, commitHash string) ([]*Redis, error) {
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
func WriteJSONToCaches(ctx context.Context, cacheClients []*Redis, key string, value interface{}, TTLSeconds uint) error {
	var g errgroup.Group
	for _, cacheClient := range cacheClients {
		func(ch *Redis) {
			g.Go(func() error {
				return ch.SetJSON(ctx, key, value, TTLSeconds)
			})
		}(cacheClient)
	}

	return g.Wait()
}

func connectToInstance(clients chan *Redis, address string, commitHash string) error {
	redisClient, err := NewRedisClient(RedisClientOptions{
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
