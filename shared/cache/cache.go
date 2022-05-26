package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Pocket/global-services/shared/environment"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/go-redis/redis/v8"
)

var poolSize = int(environment.GetInt64("CACHE_CONNECTION_POOL_SIZE", 10))

// RedisClientOptions is the struct for the baseOptions of the library client and
// any additional one for the Redis struct
type RedisClientOptions struct {
	BaseOptions *redis.Options
	KeyPrefix   string
	Name        string
}

// Redis represents a redis client with key prefix for all the implemented operations
type Redis struct {
	Client    redis.Cmdable
	KeyPrefix string
	Name      string
}

// NewRedisClient returns a client for a non-cluster instance
func NewRedisClient(ctx context.Context, options *RedisClientOptions) (*Redis, error) {
	return connectToRedis(ctx, redis.NewClient(options.BaseOptions), options)
}

// NewRedisClusterClient returns a client for a cluster instance
func NewRedisClusterClient(ctx context.Context, options *RedisClientOptions) (*Redis, error) {
	return connectToRedis(ctx, redis.NewClusterClient(&redis.ClusterOptions{
		PoolSize: poolSize,
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
		Name:      options.Name,
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

// Addrs returns the address used for the connection of the client
func (r *Redis) Addrs() []string {
	switch client := r.Client.(type) {
	case *redis.Client:
		return []string{client.Options().Addr}
	case *redis.ClusterClient:
		return client.Options().Addrs
	}
	return []string{}
}

// WriteJSONToCaches writes the given key/values to multiple cache clients at the same time
func WriteJSONToCaches(ctx context.Context, cacheClients []*Redis, key string, value interface{}, TTLSeconds uint) error {
	return utils.RunFnOnSliceSingleFailure(cacheClients, func(ins *Redis) error {
		return ins.SetJSON(ctx, key, value, TTLSeconds)
	})
}

// UnmarshallJSONResult asserts the result and umarshalls a value expected to be JSON
func UnmarshallJSONResult(data any, err error, v any) error {
	err = assertCacheResponse(data, err)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data.(string)), v)
}

// GetStringResult asserts the result and returns a value expected to be a string
func GetStringResult(data any, err error) (string, error) {
	err = assertCacheResponse(data, err)
	if err != nil && err != ErrEmptyValue {
		return "", err
	}
	return data.(string), err
}

// PipeOperation represents and operation made over a list of items using a Redis pipeline
func (r *Redis) PipeOperation(ctx context.Context, items []*Item, cmd func(redis.Pipeliner, *Item) error) ([]redis.Cmder, error) {
	pipe := r.Client.Pipeline()
	for _, item := range items {
		cmd(pipe, item)
	}
	return pipe.Exec(ctx)
}

// MGetPipe performs a mget operation using pipeline, this is to avoid the
// CROSSLOT error on redis
func (r *Redis) MGetPipe(ctx context.Context, keys []string) ([]string, error) {
	items := []*Item{}
	for _, key := range keys {
		items = append(items, &Item{
			Key: key,
		})
	}
	pipe, err := r.PipeOperation(ctx, items, func(pipe redis.Pipeliner, it *Item) error {
		return pipe.Get(ctx, it.Key).Err()
	})

	if err != nil && err != redis.Nil {
		return nil, err
	}
	results := []string{}
	for _, result := range pipe {
		switch res := result.(type) {
		case *redis.StringCmd:
			value, _ := res.Result()
			results = append(results, value)
		}
	}
	return results, nil
}

// assertCacheResponse checks whether the result returns a valid response, guaranteed
// that any valid response will come as a string
func assertCacheResponse(val any, err error) error {
	switch {
	case err == redis.Nil || val == nil:
		return ErrKeyDoesNotExist
	case err != nil:
		return err
	case val == "":
		return ErrEmptyValue
	}

	return nil
}
