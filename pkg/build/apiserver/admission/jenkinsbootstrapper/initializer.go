package jenkinsbootstrapper

import (
	"k8s.io/apiserver/pkg/admission"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

// WantsJenkinsPipelineConfig gives access to the JenkinsPipelineConfig.  This is a historical oddity.
// It's likely that what we really wanted was this as an admission plugin config
type WantsJenkinsPipelineConfig interface {
	SetJenkinsPipelineConfig(jenkinsConfig configapi.JenkinsPipelineConfig)
	admission.InitializationValidator
}

type PluginInitializer struct {
	JenkinsPipelineConfig configapi.JenkinsPipelineConfig
}

// Initialize will check the initialization interfaces implemented by each plugin
// and provide the appropriate initialization data
func (i *PluginInitializer) Initialize(plugin admission.Interface) {
	if wantsJenkinsPipelineConfig, ok := plugin.(WantsJenkinsPipelineConfig); ok {
		wantsJenkinsPipelineConfig.SetJenkinsPipelineConfig(i.JenkinsPipelineConfig)
	}
}

// Validate will call the Validate function in each plugin if they implement
// the Validator interface.
func Validate(plugins []admission.Interface) error {
	for _, plugin := range plugins {
		if validater, ok := plugin.(admission.InitializationValidator); ok {
			err := validater.ValidateInitialization()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
