package openshiftapiserver

import (
	corev1informers "k8s.io/client-go/informers/core/v1"
	rbacinformers "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	rbacauthorizer "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
)

func NewClusterQuotaMappingController(nsInternalInformer corev1informers.NamespaceInformer, cqInternalInformer quotainformer.ClusterResourceQuotaInformer) *clusterquotamapping.ClusterQuotaMappingController {
	return clusterquotamapping.NewClusterQuotaMappingController(nsInternalInformer, cqInternalInformer)
}

func NewProjectCache(nsInformer corev1informers.NamespaceInformer, privilegedLoopbackConfig *restclient.Config, defaultNodeSelector string) (*projectcache.ProjectCache, error) {
	kubeClient, err := kubernetes.NewForConfig(privilegedLoopbackConfig)
	if err != nil {
		return nil, err
	}
	return projectcache.NewProjectCache(
			nsInformer.Informer(),
			kubeClient.CoreV1().Namespaces(),
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
