package admission

import (
	"k8s.io/kubernetes/pkg/admission"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/project/cache"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

type PluginInitializer struct {
	InternalRegistryClientFactory quotautil.InternalRegistryClientFactory
	OpenshiftClient               client.Interface
	ProjectCache                  *cache.ProjectCache
}

// Initialize will check the initialization interfaces implemented by each plugin
// and provide the appropriate initialization data
func (i *PluginInitializer) Initialize(plugins []admission.Interface) {
	for _, plugin := range plugins {
		if wantsInternalRegistryClientFactory, ok := plugin.(WantsInternalRegistryClientFactory); ok {
			wantsInternalRegistryClientFactory.SetInternalRegistryClientFactory(i.InternalRegistryClientFactory)
		}
		if wantsOpenshiftClient, ok := plugin.(WantsOpenshiftClient); ok {
			wantsOpenshiftClient.SetOpenshiftClient(i.OpenshiftClient)
		}
		if wantsProjectCache, ok := plugin.(WantsProjectCache); ok {
			wantsProjectCache.SetProjectCache(i.ProjectCache)
		}
	}
}

// Validate will call the Validate function in each plugin if they implement
// the Validator interface.
func Validate(plugins []admission.Interface) error {
	for _, plugin := range plugins {
		if validater, ok := plugin.(Validator); ok {
			err := validater.Validate()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
