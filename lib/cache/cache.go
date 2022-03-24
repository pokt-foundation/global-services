package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Pocket/global-dispatcher/common/environment"
	"github.com/go-redis/redis/v8"
)

var actionTimeout = int(environment.GetInt64("CACHE_ACTION_TIMEOUT", 5))
var poolsize = int(environment.GetInt64("CACHE_CONNECTION_POOL_SIZE", 10))

// RedisClientOptions is the struct for the baseOptions of the library client and
// any additional one for the Redis struct
type RedisClientOptions struct {
	BaseOptions *redis.Options
	KeyPrefix   string
}

// Redis represents a redis client with key prefix for all the implemented operations
type Redis struct {
	Client    redis.Cmdable
	KeyPrefix string
}

// NewRedisClient returns a client for a non-cluster instance
func NewRedisClient(options RedisClientOptions) (*Redis, error) {
	return connectToRedis(redis.NewClient(options.BaseOptions), options.KeyPrefix)
}

// NewRedisClient returns a client for a cluster instance
func NewRedisClusterClient(options RedisClientOptions) (*Redis, error) {
	return connectToRedis(redis.NewClusterClient(&redis.ClusterOptions{
		PoolSize: poolsize,
		Addrs:    []string{options.BaseOptions.Addr},
	}), options.KeyPrefix)
}

// connectToRedis pings the given client to confirm the connection is successful
func connectToRedis(client redis.Cmdable, keyPrefix string) (*Redis, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(actionTimeout)*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return &Redis{
		Client:    client,
		KeyPrefix: keyPrefix,
	}, nil
}

// SetJSON sets the value given on the cache as JSON
func (r *Redis) SetJSON(ctx context.Context, key string, value interface{}, TTLSeconds uint) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.Client.Set(ctx, r.KeyPrefix+key, jsonValue, time.Second*time.Duration(TTLSeconds)).Err()
}

// Close closes a redis client depending on whether is part of a cluster or not
func (r *Redis) Close() error {
	switch client := r.Client.(type) {
	case *redis.Client:
		return client.Close()
	case *redis.ClusterClient:
		return client.Close()
	}
	return errors.New("invalid redis client type")
}
