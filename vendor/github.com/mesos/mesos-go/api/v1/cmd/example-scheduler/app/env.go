package app

import (
	"os"
	"strconv"
	"time"
)

func env(key, defaultValue string) (value string) {
	if value = os.Getenv(key); value == "" {
		value = defaultValue
	}
	return
}

func envInt(key, defaultValue string) int {
	value, err := strconv.Atoi(env(key, defaultValue))
	if err != nil {
		panic(err.Error())
	}
	return value
}

func envDuration(key, defaultValue string) time.Duration {
	value, err := time.ParseDuration(env(key, defaultValue))
	if err != nil {
		panic(err.Error())
	}
	return value
}

func envFloat(key, defaultValue string) float64 {
	value, err := strconv.ParseFloat(env(key, defaultValue), 64)
	if err != nil {
		panic(err.Error())
	}
	return value
}
