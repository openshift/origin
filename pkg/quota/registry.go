package quota

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/install"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/quota/image"
)

// NewOriginQuotaRegistry returns a registry object that knows how to evaluate quota usage of OpenShift
// resources.
func NewOriginQuotaRegistry(osClient osclient.Interface) kquota.Registry {
	return image.NewImageQuotaRegistry(osClient)
}

// NewAllResourceQuotaRegistry returns a registry object that knows how to evaluate all resources
func NewAllResourceQuotaRegistry(osClient osclient.Interface, kubeClientSet clientset.Interface) kquota.Registry {
	return kquota.UnionRegistry{install.NewRegistry(kubeClientSet), NewOriginQuotaRegistry(osClient)}
}

// AllEvaluatedGroupKinds is the list of all group kinds that we evaluate for quotas in openshift and kube
var AllEvaluatedGroupKinds = []unversioned.GroupKind{
	kapi.Kind("Pod"),
	kapi.Kind("Service"),
	kapi.Kind("ReplicationController"),
	kapi.Kind("PersistentVolumeClaim"),
	kapi.Kind("Secret"),
	kapi.Kind("ConfigMap"),

	imageapi.Kind("ImageStream"),
}
