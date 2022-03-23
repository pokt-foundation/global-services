package utils

import (
	"math/rand"
	"time"
)

func Shuffle[T any](items []*T) []*T {
	itemsCopy := make([]*T, len(items))
	copy(itemsCopy, items)

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(itemsCopy), func(i, j int) { itemsCopy[i], itemsCopy[j] = itemsCopy[j], itemsCopy[i] })

	return itemsCopy
}
