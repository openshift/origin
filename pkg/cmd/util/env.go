package util

import (
	"fmt"
	"os"
	"strconv"
	"sync"
)

var lock sync.Mutex

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

// ThreadSafeSetEnv provides locking around the call to os.Setenv
// do an internet search like "golang setenv thread safe" and
// read about the glibc stuff
func ThreadSafeSetEnv(key, value string) error {
	lock.Lock()
	defer lock.Unlock()
	return os.Setenv(key, value)
}

// ThreadSafeUnSetEnv provides locking around the call to os.Setenv
// do an internet search like "golang setenv thread safe" and
// read about the glibc stuff
func ThreadSafeUnSetEnv(key string) error {
	lock.Lock()
	defer lock.Unlock()
	return os.Unsetenv(key)
}
