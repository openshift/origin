package admission

import (
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	restclient "k8s.io/client-go/rest"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kubeapiserveradmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	"k8s.io/kubernetes/pkg/quota"

	authorizationclient "github.com/openshift/client-go/authorization/clientset/versioned"
	buildclient "github.com/openshift/client-go/build/clientset/versioned"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
)

type PluginInitializer struct {
	OpenshiftInternalAuthorizationClient authorizationclient.Interface
	OpenshiftInternalBuildClient         buildclient.Interface
	OpenshiftInternalImageClient         imageclient.Interface
	OpenshiftInternalQuotaClient         quotaclient.Interface
	OpenshiftInternalTemplateClient      templateclient.Interface
	OpenshiftInternalUserClient          userclient.Interface
	ProjectCache                         *cache.ProjectCache
	OriginQuotaRegistry                  quota.Registry
	Authorizer                           kauthorizer.Authorizer
	JenkinsPipelineConfig                configapi.JenkinsPipelineConfig
	RESTClientConfig                     restclient.Config
	Informers                            kinternalinformers.SharedInformerFactory
	ClusterResourceQuotaInformer         quotainformer.ClusterResourceQuotaInformer
	ClusterQuotaMapper                   clusterquotamapping.ClusterQuotaMapper
	RegistryHostnameRetriever            imageapi.RegistryHostnameRetriever
	SecurityInformers                    securityinformer.SharedInformerFactory
	UserInformers                        userinformer.SharedInformerFactory
}

// Initialize will check the initialization interfaces implemented by each plugin
// and provide the appropriate initialization data
func (i *PluginInitializer) Initialize(plugin admission.Interface) {
	if wantsOpenshiftAuthorizationClient, ok := plugin.(WantsOpenshiftInternalAuthorizationClient); ok {
		wantsOpenshiftAuthorizationClient.SetOpenshiftInternalAuthorizationClient(i.OpenshiftInternalAuthorizationClient)
	}
	if wantsOpenshiftBuildClient, ok := plugin.(WantsOpenshiftInternalBuildClient); ok {
		wantsOpenshiftBuildClient.SetOpenshiftInternalBuildClient(i.OpenshiftInternalBuildClient)
	}
	if wantsOpenshiftImageClient, ok := plugin.(WantsOpenshiftInternalImageClient); ok {
		wantsOpenshiftImageClient.SetOpenshiftInternalImageClient(i.OpenshiftInternalImageClient)
	}
	if wantsOpenshiftQuotaClient, ok := plugin.(WantsOpenshiftInternalQuotaClient); ok {
		wantsOpenshiftQuotaClient.SetOpenshiftInternalQuotaClient(i.OpenshiftInternalQuotaClient)
	}
	if WantsOpenshiftInternalTemplateClient, ok := plugin.(WantsOpenshiftInternalTemplateClient); ok {
		WantsOpenshiftInternalTemplateClient.SetOpenshiftInternalTemplateClient(i.OpenshiftInternalTemplateClient)
	}
	if WantsOpenshiftInternalUserClient, ok := plugin.(WantsOpenshiftInternalUserClient); ok {
		WantsOpenshiftInternalUserClient.SetOpenshiftInternalUserClient(i.OpenshiftInternalUserClient)
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
	if kubeWantsAuthorizer, ok := plugin.(initializer.WantsAuthorizer); ok {
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
