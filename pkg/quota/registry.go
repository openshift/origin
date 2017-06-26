package quota

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kexternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/install"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinternalversion "github.com/openshift/origin/pkg/image/generated/informers/internalversion/image/internalversion"
	"github.com/openshift/origin/pkg/quota/image"
)

// NewOriginQuotaRegistry returns a registry object that knows how to evaluate quota usage of OpenShift
// resources.
func NewOriginQuotaRegistry(isInformer imageinternalversion.ImageStreamInformer, osClient osclient.Interface) kquota.Registry {
	return image.NewImageQuotaRegistry(isInformer, osClient)
}

// NewAllResourceQuotaRegistry returns a registry object that knows how to evaluate all resources
func NewAllResourceQuotaRegistry(informerFactory kexternalinformers.SharedInformerFactory, isInformer imageinternalversion.ImageStreamInformer, osClient osclient.Interface, kubeClientSet clientset.Interface) kquota.Registry {
	return kquota.UnionRegistry{install.NewRegistry(kubeClientSet, informerFactory), NewOriginQuotaRegistry(isInformer, osClient)}
}

// NewOriginQuotaRegistryForAdmission returns a registry object that knows how to evaluate quota usage of OpenShift
// resources.
// This is different that is used for reconciliation because admission has to check all forms of a resource (legacy and groupified), but
// reconciliation only has to check one.
func NewOriginQuotaRegistryForAdmission(isInformer imageinternalversion.ImageStreamInformer, osClient osclient.Interface) kquota.Registry {
	return image.NewImageQuotaRegistryForAdmission(isInformer, osClient)
}

// NewAllResourceQuotaRegistryForAdmission returns a registry object that knows how to evaluate all resources for *admission*.
// This is different that is used for reconciliation because admission has to check all forms of a resource (legacy and groupified), but
// reconciliation only has to check one.
func NewAllResourceQuotaRegistryForAdmission(informerFactory kexternalinformers.SharedInformerFactory, isInformer imageinternalversion.ImageStreamInformer, osClient osclient.Interface, kubeClientSet clientset.Interface) kquota.Registry {
	return kquota.UnionRegistry{install.NewRegistry(kubeClientSet, informerFactory), NewOriginQuotaRegistryForAdmission(isInformer, osClient)}
}

// AllEvaluatedGroupKinds is the list of all group kinds that we evaluate for quotas in openshift and kube
var AllEvaluatedGroupKinds = []schema.GroupKind{
	kapi.Kind("Pod"),
	kapi.Kind("Service"),
	kapi.Kind("ReplicationController"),
	kapi.Kind("PersistentVolumeClaim"),
	kapi.Kind("Secret"),
	kapi.Kind("ConfigMap"),

	imageapi.Kind("ImageStream"),
	imageapi.LegacyKind("ImageStream"),
}
