package util

import (
	"github.com/docker/engine-api/types/container"
)

// SafeForLoggingContainerConfig returns a copy of the container.Config object
// with sensitive information (proxy environment variables containing credentials)
// redacted.
func SafeForLoggingContainerConfig(config *container.Config) *container.Config {
	strippedEnv := SafeForLoggingEnv(config.Env)
	newConfig := *config
	newConfig.Env = strippedEnv
	return &newConfig
}
