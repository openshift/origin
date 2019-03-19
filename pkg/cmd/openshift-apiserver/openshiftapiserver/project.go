package openshiftapiserver

import (
	corev1informers "k8s.io/client-go/informers/core/v1"
	rbacinformers "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	rbacauthorizer "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	quotainformer "github.com/openshift/client-go/quota/informers/externalversions/quota/v1"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
)

func NewClusterQuotaMappingController(nsInternalInformer corev1informers.NamespaceInformer, clusterQuotaInformer quotainformer.ClusterResourceQuotaInformer) *clusterquotamapping.ClusterQuotaMappingController {
	return clusterquotamapping.NewClusterQuotaMappingController(nsInternalInformer, clusterQuotaInformer)
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

func NewProjectAuthorizationCache(subjectLocator rbacauthorizer.SubjectLocator, namespaces corev1informers.NamespaceInformer, rbacInformers rbacinformers.Interface) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		namespaces.Lister(),
		namespaces.Informer(),
		projectauth.NewAuthorizerReviewer(subjectLocator),
		rbacInformers,
	)
}
