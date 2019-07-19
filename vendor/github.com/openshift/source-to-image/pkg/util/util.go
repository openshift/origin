package util

import (
	"github.com/docker/docker/api/types/container"

	utillog "github.com/openshift/source-to-image/pkg/util/log"
)

var log = utillog.StderrLog

// SafeForLoggingContainerConfig returns a copy of the container.Config object
// with sensitive information (proxy environment variables containing credentials)
// redacted.
func SafeForLoggingContainerConfig(config *container.Config) *container.Config {
	strippedEnv := SafeForLoggingEnv(config.Env)
	newConfig := *config
	newConfig.Env = strippedEnv
	return &newConfig
}
