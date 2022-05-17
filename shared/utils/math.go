package utils

import "golang.org/x/exp/constraints"

func GetSliceAvg[T constraints.Integer | constraints.Float](slice []T) float64 {
	total := 0.0
	for _, elem := range slice {
		total += float64(elem)
	}
	return total / float64(len(slice))
}
