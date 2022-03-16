package cache

import (
	"context"
	"encoding/json"
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
		Addrs: []string{options.BaseOptions.Addr},
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
