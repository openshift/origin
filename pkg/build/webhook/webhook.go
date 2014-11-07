package webhook

import (
	"strings"
)

// GitRefMatches determines if the ref from a webhook event matches a build configuration
func GitRefMatches(eventRef, configRef string) bool {
	const RefPrefix = "refs/heads/"
	eventRef = strings.TrimPrefix(eventRef, RefPrefix)
	configRef = strings.TrimPrefix(configRef, RefPrefix)
	if configRef == "" {
		configRef = "master"
	}
	return configRef == eventRef
}
