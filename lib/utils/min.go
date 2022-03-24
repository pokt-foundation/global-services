package utils

import (
	"golang.org/x/exp/constraints"
)

// Min returns the minimum of two values
func Min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}
