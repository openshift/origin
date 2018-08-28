package admission

import (
	"k8s.io/apiserver/pkg/admission"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/quota"

	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
	securityinformer "github.com/openshift/origin/pkg/security/generated/informers/internalversion"
)

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

// WantsRESTClientConfig gives access to a RESTClientConfig.  It's useful for doing unusual things with transports.
type WantsRESTClientConfig interface {
	SetRESTClientConfig(restclient.Config)
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
