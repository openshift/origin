package legacy

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	quotav1 "github.com/openshift/api/quota/v1"
)

func InstallExternalLegacyQuota(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedQuotaTypes,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedQuotaTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&quotav1.ClusterResourceQuota{},
		&quotav1.ClusterResourceQuotaList{},
		&quotav1.AppliedClusterResourceQuota{},
		&quotav1.AppliedClusterResourceQuotaList{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}
