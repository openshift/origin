package webhook

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/openshift/origin/pkg/api/latest"
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
		apiVersion := latest.Version
		if accessor, err := meta.Accessor(bc); err == nil && len(accessor.APIVersion()) > 0 {
			apiVersion = accessor.APIVersion()
		}
		url := fmt.Sprintf("%s/osapi/%s/buildConfigHooks/%s/%s/%s", config.Host, apiVersion, bc.Name, whTrigger, bc.Triggers[i].Type)
		triggers = append(triggers, url)
	}
	return triggers
}
