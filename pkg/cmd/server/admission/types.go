package admission

import (
	"k8s.io/apiserver/pkg/admission"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	restclient "k8s.io/client-go/rest"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/quota"

	authorizationclient "github.com/openshift/client-go/authorization/clientset/versioned"
	buildclient "github.com/openshift/client-go/build/clientset/versioned"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
)

type WantsOpenshiftInternalAuthorizationClient interface {
	SetOpenshiftInternalAuthorizationClient(authorizationclient.Interface)
	admission.InitializationValidator
}

type WantsOpenshiftInternalBuildClient interface {
	SetOpenshiftInternalBuildClient(buildclient.Interface)
	admission.InitializationValidator
}

// WantsOpenshiftInternalQuotaClient should be implemented by admission plugins that need
// an Openshift internal quota client
type WantsOpenshiftInternalQuotaClient interface {
	SetOpenshiftInternalQuotaClient(quotaclient.Interface)
	admission.InitializationValidator
}

// WantsOpenshiftInternalUserClient should be implemented by admission plugins that need
// an Openshift internal user client
type WantsOpenshiftInternalUserClient interface {
	SetOpenshiftInternalUserClient(userclient.Interface)
	admission.InitializationValidator
}

// WantsOpenshiftInternalImageClient should be implemented by admission plugins that need
// an Openshift internal image client
type WantsOpenshiftInternalImageClient interface {
	SetOpenshiftInternalImageClient(imageclient.Interface)
	admission.InitializationValidator
}

// WantsOpenshiftInternalTemplateClient should be implemented by admission plugins that need
// an Openshift internal template client
type WantsOpenshiftInternalTemplateClient interface {
	SetOpenshiftInternalTemplateClient(templateclient.Interface)
	admission.InitializationValidator
}

// WantsProjectCache should be implemented by admission plugins that need a
// project cache
type WantsProjectCache interface {
	SetProjectCache(*cache.ProjectCache)
	admission.InitializationValidator
}

// WantsQuotaRegistry should be implemented by admission plugins that need a quota registry
type WantsOriginQuotaRegistry interface {
	SetOriginQuotaRegistry(quota.Registry)
	admission.InitializationValidator
}

// WantsAuthorizer should be implemented by admission plugins that
// need access to the Authorizer interface
type WantsAuthorizer interface {
	SetAuthorizer(kauthorizer.Authorizer)
	admission.InitializationValidator
}

// WantsJenkinsPipelineConfig gives access to the JenkinsPipelineConfig.  This is a historical oddity.
// It's likely that what we really wanted was this as an admission plugin config
type WantsJenkinsPipelineConfig interface {
	SetJenkinsPipelineConfig(jenkinsConfig configapi.JenkinsPipelineConfig)
	admission.InitializationValidator
}

// WantsRESTClientConfig gives access to a RESTClientConfig.  It's useful for doing unusual things with transports.
type WantsRESTClientConfig interface {
	SetRESTClientConfig(restclient.Config)
	admission.InitializationValidator
}

// WantsInternalKubernetesInformers should be implemented by admission plugins that need the internal kubernetes
// informers.
type WantsInternalKubernetesInformers interface {
	SetInternalKubernetesInformers(kinternalinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsClusterQuota should be implemented by admission plugins that need to know how to map between
// cluster quota and namespaces and get access to the informer.
type WantsClusterQuota interface {
	SetClusterQuota(clusterquotamapping.ClusterQuotaMapper, quotainformer.ClusterResourceQuotaInformer)
	admission.InitializationValidator
}

type WantsSecurityInformer interface {
	SetSecurityInformers(securityinformer.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsDefaultRegistryFunc should be implemented by admission plugins that need to know the default registry
// address.
type WantsDefaultRegistryFunc interface {
	SetDefaultRegistryFunc(func() (string, bool))
	admission.InitializationValidator
}

type WantsUserInformer interface {
	SetUserInformer(userinformer.SharedInformerFactory)
	admission.InitializationValidator
}
