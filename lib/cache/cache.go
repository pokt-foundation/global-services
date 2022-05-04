package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Pocket/global-services/common/environment"
	"github.com/go-redis/redis/v8"
)

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
func NewRedisClient(ctx context.Context, options *RedisClientOptions) (*Redis, error) {
	return connectToRedis(ctx, redis.NewClient(options.BaseOptions), options)
}

// NewRedisClient returns a client for a cluster instance
func NewRedisClusterClient(ctx context.Context, options *RedisClientOptions) (*Redis, error) {
	return connectToRedis(ctx, redis.NewClusterClient(&redis.ClusterOptions{
		PoolSize: poolsize,
		Addrs:    []string{options.BaseOptions.Addr},
	}), options)
}

// connectToRedis pings the given client to confirm the connection is successful
func connectToRedis(ctx context.Context, client redis.Cmdable, options *RedisClientOptions) (*Redis, error) {
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return &Redis{
		Client:    client,
		KeyPrefix: options.KeyPrefix,
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
