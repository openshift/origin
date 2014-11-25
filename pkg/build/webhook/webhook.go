package webhook

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/openshift/origin/pkg/build/api"
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

// FindTrigger retrieves the BuildTrigger of a given type from a build configuration
func FindTriggerPolicy(triggerType api.BuildTriggerType, config *api.BuildConfig) (*api.BuildTriggerPolicy, bool) {
	for _, p := range config.Triggers {
		if p.Type == triggerType {
			return &p, true
		}
	}
	return nil, false
}

// GetWebhookUrl assembles array of webhook urls which can trigger given buildConfig
func GetWebhookUrl(bc *api.BuildConfig, config *client.Config) []string {
	triggers := make([]string, len(bc.Triggers))
	for i, trigger := range bc.Triggers {
		whTrigger := ""
		switch trigger.Type {
		case "github":
			whTrigger = trigger.GithubWebHook.Secret
		case "generic":
			whTrigger = trigger.GenericWebHook.Secret
		}
		urlPath := fmt.Sprintf("osapi/%s/buildConfigHooks/%s/%s/%s", bc.APIVersion, bc.Name, whTrigger, bc.Triggers[i].Type)
		u := url.URL{
			Scheme: "http",
			Host:   "host",
			Path:   urlPath,
		}
		triggers = append(triggers, u.String())
	}
	return triggers
}
