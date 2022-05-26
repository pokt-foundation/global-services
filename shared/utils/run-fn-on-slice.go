package utils

import (
	"sync"

	"golang.org/x/sync/errgroup"
)

// RunFnOnSliceSingleFailure allows to run a function on all the elements of a
// slice, having access to the individual elements of the slice.
// returns err if any of them fails
func RunFnOnSliceSingleFailure[T any](slice []T, fn func(T) error) error {
	var g errgroup.Group
	for _, element := range slice {
		func(el T) {
			g.Go(func() error {
				return fn(el)
			})
		}(element)
	}
	return g.Wait()
}

// RunFnOnSliceMultipleFailures allows to run a function on all the elements of a
// slice, having access to the individual elements of the slice.
// returns individual errors of the functions
func RunFnOnSliceMultipleFailures[T any](slice []T, fn func(T) error) []error {
	errs := make([]error, len(slice))

	var wg sync.WaitGroup
	for index, element := range slice {
		wg.Add(1)

		go func(idx int, el T) {
			defer wg.Done()
			if err := fn(el); err != nil {
				errs[idx] = err
			}
		}(index, element)
	}
	wg.Wait()

	return errs
}
