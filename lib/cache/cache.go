package cache

import (
	"github.com/go-redis/redis/v8"
)

type RedisClient struct {
	client redis.Cmdable
}
