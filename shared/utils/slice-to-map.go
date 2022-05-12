package utils

// SliceToMappedStruct takes a slice and returns a string/T map with the key
// being a field of the struct
func SliceToMappedStruct[T any](slice []*T, fn func(*T) string) map[string]*T {
	mapFromSlice := make(map[string]*T)
	for _, value := range slice {
		mapFromSlice[fn(value)] = value
	}
	return mapFromSlice
}
