package admission

import (
	"k8s.io/apiserver/pkg/admission"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/quota"

	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
)

type PluginInitializer struct {
	ProjectCache                 *cache.ProjectCache
	OriginQuotaRegistry          quota.Registry
	RESTClientConfig             restclient.Config
	ClusterResourceQuotaInformer quotainformer.ClusterResourceQuotaInformer
	ClusterQuotaMapper           clusterquotamapping.ClusterQuotaMapper
	RegistryHostnameRetriever    registryhostname.RegistryHostnameRetriever
	SecurityInformers            securityinformer.SharedInformerFactory
	UserInformers                userinformer.SharedInformerFactory
}

// Initialize will check the initialization interfaces implemented by each plugin
// and provide the appropriate initialization data
func (i *PluginInitializer) Initialize(plugin admission.Interface) {
	if wantsProjectCache, ok := plugin.(WantsProjectCache); ok {
		wantsProjectCache.SetProjectCache(i.ProjectCache)
	}
	if wantsOriginQuotaRegistry, ok := plugin.(WantsOriginQuotaRegistry); ok {
		wantsOriginQuotaRegistry.SetOriginQuotaRegistry(i.OriginQuotaRegistry)
	}
	if wantsRESTClientConfig, ok := plugin.(WantsRESTClientConfig); ok {
		wantsRESTClientConfig.SetRESTClientConfig(i.RESTClientConfig)
	}
	if wantsClusterQuota, ok := plugin.(WantsClusterQuota); ok {
		wantsClusterQuota.SetClusterQuota(i.ClusterQuotaMapper, i.ClusterResourceQuotaInformer)
	}
	if wantsSecurityInformer, ok := plugin.(WantsSecurityInformer); ok {
		wantsSecurityInformer.SetSecurityInformers(i.SecurityInformers)
	}
	if wantsDefaultRegistryFunc, ok := plugin.(WantsDefaultRegistryFunc); ok {
		wantsDefaultRegistryFunc.SetDefaultRegistryFunc(i.RegistryHostnameRetriever.InternalRegistryHostname)
	}
	if wantsUserInformer, ok := plugin.(WantsUserInformer); ok {
		wantsUserInformer.SetUserInformer(i.UserInformers)
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
