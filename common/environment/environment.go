package environment

import (
	"os"
	"strconv"
	"strings"
)

// GetString gets the environment var as a string
func GetString(varName string, defaultValue string) string {
	val, _ := os.LookupEnv(varName)
	if val == "" {
		return defaultValue
	}

	return val
}

// GetInt64 gets the env var as an int
func GetInt64(varName string, defaultValue int64) int64 {
	val, ok := os.LookupEnv(varName)
	if !ok {
		return defaultValue
	}

	iVal, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return defaultValue
	}

	return iVal
}

// GetBoolean gets the env var as a boolean
func GetBool(varName string, defaultValue bool) bool {
	val, _ := os.LookupEnv(varName)
	if strings.ToLower(val) == "true" {
		return true
	}
	if strings.ToLower(val) == "false" {
		return false
	}

	return defaultValue
}
