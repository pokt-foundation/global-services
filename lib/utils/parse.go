package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// ParseIntegerFromPayload parses a string value as an int from a JSON payload
// Should this be refactored to also support strings?
func ParseIntegerFromPayload(r io.Reader, key string) (int64, error) {
	// TODO: Parse nested fields
	res := map[string]any{} // got'em 'any' haters

	if err := json.NewDecoder(r).Decode(&res); err != nil {
		return 0, errors.New("error decoding payload: " + err.Error())
	}

	value, ok := res[key]
	if !ok {
		return 0, errors.New("key not found in payload, key: " + key)
	}

	valueStr, ok := value.(string)
	if !ok {
		return 0, errors.New("invalid cast for field: " + key)
	}

	valueDecimal, err := strconv.ParseInt(valueStr, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing field %s: %s", key, err.Error())
	}

	return valueDecimal, nil
}
