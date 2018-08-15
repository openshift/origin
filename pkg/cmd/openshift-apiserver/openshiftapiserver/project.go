package openshiftapiserver

import (
	rbacinformers "k8s.io/client-go/informers/rbac/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/core/internalversion"
	rbacauthorizer "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
)

func NewClusterQuotaMappingController(nsInternalInformer kinternalinformers.NamespaceInformer, cqInternalInformer quotainformer.ClusterResourceQuotaInformer) *clusterquotamapping.ClusterQuotaMappingController {
	return clusterquotamapping.NewClusterQuotaMappingControllerInternal(nsInternalInformer, cqInternalInformer)
}

func NewProjectCache(nsInternalInformer kinternalinformers.NamespaceInformer, privilegedLoopbackConfig *restclient.Config, defaultNodeSelector string) (*projectcache.ProjectCache, error) {
	kubeInternalClient, err := kclientsetinternal.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}
	return projectcache.NewProjectCache(
			nsInternalInformer.Informer(),
			kubeInternalClient.Core().Namespaces(),
			defaultNodeSelector),
		nil
}

func NewProjectAuthorizationCache(subjectLocator rbacauthorizer.SubjectLocator, namespaces cache.SharedIndexInformer, rbacInformers rbacinformers.Interface) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		namespaces,
		projectauth.NewAuthorizerReviewer(subjectLocator),
		rbacInformers,
	)
}
