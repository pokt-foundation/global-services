package utils

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/pokt-foundation/pocket-go/pkg/relayer"
)

func GetIntFromRelay(pocketRelayer relayer.PocketRelayer, input relayer.RelayInput, key string) (int64, error) {
	relay, err := pocketRelayer.Relay(&input, nil)
	if err != nil {
		return 0, errors.New("error relaying: " + err.Error())
	}

	result, err := ParseIntegerFromPayload(
		bytes.NewReader([]byte(relay.RelayOutput.Response)), key)
	if err != nil {
		return 0, fmt.Errorf("error parsing key %s: %s", key, err.Error())
	}

	return result, nil
}
