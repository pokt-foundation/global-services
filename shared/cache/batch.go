package cache

import (
	"context"
	"sync"
	"time"

	logger "github.com/Pocket/global-services/shared/logger"
	"github.com/Pocket/global-services/shared/utils"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

// Item represents a single item to be written on the batch
type Item struct {
	Key   string
	Value interface{}
	TTL   time.Duration
}

// BatchWriterOptions is the config for the batch writer
type BatchWriterOptions struct {
	Caches    []*Redis
	BatchSize int
	WaitGroup *sync.WaitGroup
	RequestID string
}

// BatchWriter spans a monitor goroutine which is constantly checking for items to write to redis,
// once the items reached the minimum threshold it is sent to be written as a single Redis SET operation.
func BatchWriter(ctx context.Context, options *BatchWriterOptions) chan *Item {
	batch := make(chan *Item, options.BatchSize)
	go monitorBatch(ctx, batch, *options)
	return batch
}

func monitorBatch(ctx context.Context, batch chan *Item, options BatchWriterOptions) {
	defer options.WaitGroup.Done()
	items := []*Item{}

	for {
		item, ok := <-batch
		if item != nil {
			items = append(items, item)
		}

		if ok && len(items) < options.BatchSize {
			continue
		}
		writeBatch(ctx, items, options.Caches, options.RequestID)
		items = nil

		if !ok {
			break
		}
	}
}

func writeBatch(ctx context.Context, items []*Item, caches []*Redis, requestID string) {
	if err := utils.RunFnOnSliceSingleFailure(caches, func(cache *Redis) error {
		_, err := cache.PipeOperation(ctx, items, func(pipe redis.Pipeliner, it *Item) error {
			return pipe.Set(ctx, it.Key, it.Value, it.TTL).Err()
		})
		return err
	}); err != nil {
		logger.Log.WithFields(log.Fields{
			"error":     err.Error(),
			"requestID": requestID,
		}).Errorf("cache: error writing cache batch: %s", err.Error())
	}
}
