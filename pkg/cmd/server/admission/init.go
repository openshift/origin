package admission

import (
	"k8s.io/apiserver/pkg/admission"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	restclient "k8s.io/client-go/rest"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kubeapiserveradmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	"k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	usercache "github.com/openshift/origin/pkg/user/cache"
)

type PluginInitializer struct {
	OpenshiftClient              client.Interface
	ProjectCache                 *cache.ProjectCache
	OriginQuotaRegistry          quota.Registry
	Authorizer                   kauthorizer.Authorizer
	JenkinsPipelineConfig        configapi.JenkinsPipelineConfig
	RESTClientConfig             restclient.Config
	Informers                    kinternalinformers.SharedInformerFactory
	ClusterResourceQuotaInformer quotainformer.ClusterResourceQuotaInformer
	ClusterQuotaMapper           clusterquotamapping.ClusterQuotaMapper
	DefaultRegistryFn            imageapi.DefaultRegistryFunc
	GroupCache                   *usercache.GroupCache
	SecurityInformers            securityinformer.SharedInformerFactory
}

// Initialize will check the initialization interfaces implemented by each plugin
// and provide the appropriate initialization data
func (i *PluginInitializer) Initialize(plugin admission.Interface) {
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
	if kubeWantsAuthorizer, ok := plugin.(kubeapiserveradmission.WantsAuthorizer); ok {
		kubeWantsAuthorizer.SetAuthorizer(i.Authorizer)
	}
	if wantsJenkinsPipelineConfig, ok := plugin.(WantsJenkinsPipelineConfig); ok {
		wantsJenkinsPipelineConfig.SetJenkinsPipelineConfig(i.JenkinsPipelineConfig)
	}
	if wantsRESTClientConfig, ok := plugin.(WantsRESTClientConfig); ok {
		wantsRESTClientConfig.SetRESTClientConfig(i.RESTClientConfig)
	}
	if wantsInformers, ok := plugin.(WantsInternalKubernetesInformers); ok {
		wantsInformers.SetInternalKubernetesInformers(i.Informers)
	}
	if wantsInformerFactory, ok := plugin.(kubeapiserveradmission.WantsInternalKubeInformerFactory); ok {
		wantsInformerFactory.SetInternalKubeInformerFactory(i.Informers)
	}
	if wantsClusterQuota, ok := plugin.(WantsClusterQuota); ok {
		wantsClusterQuota.SetClusterQuota(i.ClusterQuotaMapper, i.ClusterResourceQuotaInformer)
	}
	if wantsSecurityInformer, ok := plugin.(WantsSecurityInformer); ok {
		wantsSecurityInformer.SetSecurityInformers(i.SecurityInformers)
	}
	if wantsDefaultRegistryFunc, ok := plugin.(WantsDefaultRegistryFunc); ok {
		wantsDefaultRegistryFunc.SetDefaultRegistryFunc(i.DefaultRegistryFn)
	}
	if wantsGroupCache, ok := plugin.(WantsGroupCache); ok {
		wantsGroupCache.SetGroupCache(i.GroupCache)
	}
}

// Validate will call the Validate function in each plugin if they implement
// the Validator interface.
func Validate(plugins []admission.Interface) error {
	for _, plugin := range plugins {
		if validater, ok := plugin.(admission.Validator); ok {
			err := validater.Validate()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
