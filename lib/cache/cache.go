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

type RedisClientOptions struct {
	BaseOptions *redis.Options
	KeyPrefix   string
}

type Redis struct {
	Client    redis.Cmdable
	KeyPrefix string
}

func NewRedisClient(options RedisClientOptions) (*Redis, error) {
	return connectToRedis(redis.NewClient(options.BaseOptions), options.KeyPrefix)
}

func NewRedisClusterClient(options RedisClientOptions) (*Redis, error) {
	return connectToRedis(redis.NewClusterClient(&redis.ClusterOptions{
		PoolSize: 200,
		Addrs:    []string{options.BaseOptions.Addr},
	}), options.KeyPrefix)
}

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

func (r *Redis) SetJSON(ctx context.Context, key string, value interface{}, TTLSeconds uint) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.Client.Set(ctx, r.KeyPrefix+key, jsonValue, time.Second*time.Duration(TTLSeconds)).Err()
}

func (r *Redis) Close() error {
	switch client := r.Client.(type) {
	case *redis.Client:
		return client.Close()
	case *redis.ClusterClient:
		return client.Close()
	}
	return errors.New("invalid redis client type")
}
