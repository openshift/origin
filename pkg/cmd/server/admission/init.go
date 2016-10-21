package admission

import (
	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/adapter"
	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/controller/shared"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
)

type PluginInitializer struct {
	OpenshiftClient       client.Interface
	ProjectCache          *cache.ProjectCache
	OriginQuotaRegistry   quota.Registry
	Authorizer            authorizer.Authorizer
	JenkinsPipelineConfig configapi.JenkinsPipelineConfig
	RESTClientConfig      restclient.Config
	Informers             shared.InformerFactory
	ClusterQuotaMapper    clusterquotamapping.ClusterQuotaMapper
	DefaultRegistryFn     imageapi.DefaultRegistryFunc
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
		if kubeWantsAuthorizer, ok := plugin.(admission.WantsAuthorizer); ok {
			kubeAuthorizer, err := adapter.NewAuthorizer(i.Authorizer)
			// this shouldn't happen
			if err != nil {
				panic(err)
			}
			kubeWantsAuthorizer.SetAuthorizer(kubeAuthorizer)
		}
		if wantsJenkinsPipelineConfig, ok := plugin.(WantsJenkinsPipelineConfig); ok {
			wantsJenkinsPipelineConfig.SetJenkinsPipelineConfig(i.JenkinsPipelineConfig)
		}
		if wantsRESTClientConfig, ok := plugin.(WantsRESTClientConfig); ok {
			wantsRESTClientConfig.SetRESTClientConfig(i.RESTClientConfig)
		}
		if wantsInformers, ok := plugin.(WantsInformers); ok {
			wantsInformers.SetInformers(i.Informers)
		}
		if wantsInformerFactory, ok := plugin.(admission.WantsInformerFactory); ok {
			wantsInformerFactory.SetInformerFactory(i.Informers.KubernetesInformers())
		}
		if wantsClusterQuotaMapper, ok := plugin.(WantsClusterQuotaMapper); ok {
			wantsClusterQuotaMapper.SetClusterQuotaMapper(i.ClusterQuotaMapper)
		}
		if wantsDefaultRegistryFunc, ok := plugin.(WantsDefaultRegistryFunc); ok {
			wantsDefaultRegistryFunc.SetDefaultRegistryFunc(i.DefaultRegistryFn)
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
