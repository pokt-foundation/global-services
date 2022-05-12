package utils

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseIntegerFromPayload(t *testing.T) {
	c := require.New(t)

	number, err := ParseIntegerFromPayload(bytes.NewReader([]byte(`{"ajua":12}`)), "ajua")
	c.NoError(err)
	c.Equal(int64(12), number)

	number, err = ParseIntegerFromPayload(bytes.NewReader([]byte(`{"ajua":"12"}`)), "ajua")
	c.NoError(err)
	c.Equal(int64(12), number)

	number, err = ParseIntegerFromPayload(bytes.NewReader([]byte(`{"ajua":["12"]}`)), "ajua")
	c.EqualError(err, "error parsing field ajua: invalid type for payload: []interface {}")
	c.Empty(number)
}

func TestParseIntegerJSONString(t *testing.T) {
	c := require.New(t)

	number, err := ParseIntegerJSONString(`{"ajua":12}`, "ajua")
	c.NoError(err)
	c.Equal(int64(12), number)

	number, err = ParseIntegerJSONString(`{"ajua":"12"}`, "ajua")
	c.NoError(err)
	c.Equal(int64(12), number)

	number, err = ParseIntegerJSONString(`{"ajua":["12"]}`, "ajua")
	c.EqualError(err, "error parsing field ajua: invalid type for payload: []interface {}")
	c.Empty(number)
}
