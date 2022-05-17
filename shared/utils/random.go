package utils

import (
	"encoding/hex"
	"math/rand"
)

// RandomHex returns a random hexadecimal string of n length
func RandomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
