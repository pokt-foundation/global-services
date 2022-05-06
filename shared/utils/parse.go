package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// ParseIntegerFromPayload parses a string (or integer if any) value as an int from a JSON payload
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

	var valueDecimal int64
	var err error
	switch v := value.(type) {
	case string:
		valueDecimal, err = strconv.ParseInt(v, 0, 64)
	case int64:
		valueDecimal = int64(v)
	case float64:
		valueDecimal = int64(v)
	default:
		err = fmt.Errorf("invalid type for payload: %T", v)
	}

	if err != nil {
		return 0, fmt.Errorf("error parsing field %s: %s", key, err.Error())
	}

	return valueDecimal, nil
}
