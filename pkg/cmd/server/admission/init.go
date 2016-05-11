package admission

import (
	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/project/cache"
)

type PluginInitializer struct {
	OpenshiftClient     client.Interface
	ProjectCache        *cache.ProjectCache
	OriginQuotaRegistry quota.Registry
	Authorizer          authorizer.Authorizer
}

// Initialize will check the initialization interfaces implemented by each plugin
// and provide the appropriate initialization data
func (i *PluginInitializer) Initialize(plugins []admission.Interface) {
	for _, plugin := range plugins {
		if wantsOpenshiftClient, ok := plugin.(WantsOpenshiftClient); ok {
			wantsOpenshiftClient.SetOpenshiftClient(i.OpenshiftClient)
		}
		if wantsProjectCache, ok := plugin.(WantsProjectCache); ok {
			wantsProjectCache.SetProjectCache(i.ProjectCache)
		}
		if wantsOriginQuotaRegistry, ok := plugin.(WantsOriginQuotaRegistry); ok {
			wantsOriginQuotaRegistry.SetOriginQuotaRegistry(i.OriginQuotaRegistry)
		}
		if wantsAuthorizer, ok := plugin.(WantsAuthorizer); ok {
			wantsAuthorizer.SetAuthorizer(i.Authorizer)
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
