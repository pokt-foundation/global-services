package cache

import (
	"context"
	"sync"
	"time"

	logger "github.com/Pocket/global-dispatcher/lib/logger"
	log "github.com/sirupsen/logrus"
)

type Item struct {
	Key   string
	Value interface{}
	TTL   time.Duration
}

type BatchWriterOptions struct {
	Caches    []*Redis
	BatchSize int
	Chan      chan *Item
	WaitGroup *sync.WaitGroup
	RequestID string
}

func BatchWriter(ctx context.Context, options BatchWriterOptions) {
	defer options.WaitGroup.Done()
	items := []*Item{}

	for x := range options.Chan {
		items = append(items, x)
		if len(items) < options.BatchSize {
			continue
		}
		WriteBatch(ctx, items, options.Caches, options.RequestID)
		items = nil
	}

	// There might be items left
	WriteBatch(ctx, items, options.Caches, options.RequestID)
}

func WriteBatch(ctx context.Context, items []*Item, caches []*Redis, requestID string) {
	if err := RunFunctionOnAllClients(caches, func(cache *Redis) error {
		pipe := cache.Client.Pipeline()
		for _, item := range items {
			pipe.Set(ctx, item.Key, item.Value, item.TTL)
		}
		_, err := pipe.Exec(ctx)
		return err
	}); err != nil {
		logger.Log.WithFields(log.Fields{
			"error":     err.Error(),
			"requestID": requestID,
		}).Errorf("cache: error writing cache batch: %s", err.Error())
	}
}
