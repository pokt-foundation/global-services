package shared

import "errors"

var (
	// ErrNoCacheClientProvided when no cache client is provided
	ErrNoCacheClientProvided = errors.New("no cache clients were provided")
)
