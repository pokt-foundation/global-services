package utils

import (
	"errors"
	"fmt"

	"github.com/pokt-foundation/pocket-go/relayer"
)

// GetIntFromRelay performs a relay which result is expected to be an int and parses the result
func GetIntFromRelay(Relayer relayer.Relayer, input relayer.Input, key string) (int64, error) {
	relay, err := Relayer.Relay(&input, nil)
	if err != nil {
		return 0, errors.New("error relaying: " + err.Error())
	}

	result, err := ParseIntegerJSONString(relay.RelayOutput.Response, key)
	if err != nil {
		return 0, fmt.Errorf("error parsing key %s: %s", key, err.Error())
	}

	return result, nil
}
