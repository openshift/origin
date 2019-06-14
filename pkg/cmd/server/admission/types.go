package admission

import (
	"k8s.io/apiserver/pkg/admission"
	restclient "k8s.io/client-go/rest"
	quota "k8s.io/kubernetes/pkg/quota/v1"

	authorizationinformer "github.com/openshift/client-go/authorization/informers/externalversions/authorization/v1"
	quotainformer "github.com/openshift/client-go/quota/informers/externalversions/quota/v1"
	securityv1informer "github.com/openshift/client-go/security/informers/externalversions/security/v1"
	userinformer "github.com/openshift/client-go/user/informers/externalversions"
	"github.com/openshift/library-go/pkg/quota/clusterquotamapping"
	"github.com/openshift/origin/pkg/project/cache"
)

// WantsProjectCache should be implemented by admission plugins that need a
// project cache
type WantsProjectCache interface {
	SetProjectCache(*cache.ProjectCache)
	admission.InitializationValidator
}

type WantsDefaultNodeSelector interface {
	SetDefaultNodeSelector(string)
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
	SetSecurityInformers(securityv1informer.SecurityContextConstraintsInformer)
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

type WantsRoleBindingRestrictionInformer interface {
	SetRoleBindingRestrictionInformer(authorizationinformer.RoleBindingRestrictionInformer)
	admission.InitializationValidator
}
