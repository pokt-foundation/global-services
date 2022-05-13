package utils

import "golang.org/x/sync/errgroup"

// RunFnOnSlice allows to run a function on all the elements of a
// slice, having access to the individual elements of the slice,
// returns err if any of them fails
func RunFnOnSlice[T any](slice []T, fn func(T) error) error {
	var g errgroup.Group
	for _, element := range slice {
		func(ch T) {
			g.Go(func() error {
				return fn(ch)
			})
		}(element)
	}
	return g.Wait()
}
