package cache

import "errors"

var (
	// ErrEmptyValue when a key returns an empty value
	ErrEmptyValue = errors.New("value from key is empty")
	// ErrKeyDoesNotExist when a key does not exist
	ErrKeyDoesNotExist = errors.New("key does not exist")
)
