package util

import (
	"fmt"
	"os"
	"strconv"
)

func EnvInt(key string, defaultValue int32, minValue int32) int32 {
	value, err := strconv.ParseInt(Env(key, fmt.Sprintf("%d", defaultValue)), 10, 32)
	if err != nil || int32(value) < minValue {
		return defaultValue
	}
	return int32(value)
}

// Env returns an environment variable or a default value if not specified.
func Env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

// GetEnv returns an environment value if specified
func GetEnv(key string) (string, bool) {
	val := os.Getenv(key)
	if len(val) == 0 {
		return "", false
	}
	return val, true
}
