package admission

import (
	"k8s.io/apiserver/pkg/admission"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	restclient "k8s.io/client-go/rest"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
	userinformer "github.com/openshift/origin/pkg/user/generated/informers/internalversion"
)

// WantsOpenshiftClient should be implemented by admission plugins that need
// an Openshift client
type WantsOpenshiftClient interface {
	SetOpenshiftClient(client.Interface)
	admission.Validator
}

// WantsProjectCache should be implemented by admission plugins that need a
// project cache
type WantsProjectCache interface {
	SetProjectCache(*cache.ProjectCache)
	admission.Validator
}

// WantsQuotaRegistry should be implemented by admission plugins that need a quota registry
type WantsOriginQuotaRegistry interface {
	SetOriginQuotaRegistry(quota.Registry)
	admission.Validator
}

// WantsAuthorizer should be implemented by admission plugins that
// need access to the Authorizer interface
type WantsAuthorizer interface {
	SetAuthorizer(kauthorizer.Authorizer)
	admission.Validator
}

// WantsJenkinsPipelineConfig gives access to the JenkinsPipelineConfig.  This is a historical oddity.
// It's likely that what we really wanted was this as an admission plugin config
type WantsJenkinsPipelineConfig interface {
	SetJenkinsPipelineConfig(jenkinsConfig configapi.JenkinsPipelineConfig)
	admission.Validator
}

// WantsRESTClientConfig gives access to a RESTClientConfig.  It's useful for doing unusual things with transports.
type WantsRESTClientConfig interface {
	SetRESTClientConfig(restclient.Config)
	admission.Validator
}

// WantsInternalKubernetesInformers should be implemented by admission plugins that need the internal kubernetes
// informers.
type WantsInternalKubernetesInformers interface {
	SetInternalKubernetesInformers(kinternalinformers.SharedInformerFactory)
	admission.Validator
}

// WantsClusterQuota should be implemented by admission plugins that need to know how to map between
// cluster quota and namespaces and get access to the informer.
type WantsClusterQuota interface {
	SetClusterQuota(clusterquotamapping.ClusterQuotaMapper, quotainformer.ClusterResourceQuotaInformer)
	admission.Validator
}

type WantsSecurityInformer interface {
	SetSecurityInformers(securityinformer.SharedInformerFactory)
	admission.Validator
}

// WantsDefaultRegistryFunc should be implemented by admission plugins that need to know the default registry
// address.
type WantsDefaultRegistryFunc interface {
	SetDefaultRegistryFunc(imageapi.DefaultRegistryFunc)
	admission.Validator
}

type WantsUserInformer interface {
	SetUserInformer(userinformer.SharedInformerFactory)
	admission.Validator
}
