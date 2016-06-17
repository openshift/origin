package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

// ClusterResourceQuota mirrors ResourceQuota at a cluster scope.  This object is easily convertible to
// synthetic ResourceQuota object to allow quota evaluation re-use.
type ClusterResourceQuota struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata"`

	// Spec defines the desired quota
	Spec ClusterResourceQuotaSpec `json:"spec"`

	// Status defines the actual enforced quota and its current usage
	Status ClusterResourceQuotaStatus `json:"status,omitempty"`
}

// ClusterResourceQuotaSpec defines the desired quota restrictions
type ClusterResourceQuotaSpec struct {
	// Selector is the label selector used to match projects.  It is not allowed to be empty
	// and should only select active projects on the scale of dozens (though it can select
	// many more less active projects).  These projects will contend on object creation through
	// this resource.
	Selector *unversioned.LabelSelector `json:"selector"`

	// Quota defines the desired quota
	Quota kapi.ResourceQuotaSpec `json:"quota"`
}

// ClusterResourceQuotaStatus defines the actual enforced quota and its current usage
type ClusterResourceQuotaStatus struct {
	// Total defines the actual enforced quota and its current usage across all namespaces
	Total kapi.ResourceQuotaStatus `json:"total"`

	// Namespaces slices the usage by namespace.  This division allows for quick resolution of
	// deletion reconcilation inside of a single namespace without requiring a recalculation
	// across all namespaces.  This can be used to pull the deltas for a given namespace.
	Namespaces ResourceQuotasStatusByNamespace `json:"namespaces"`
}

// ClusterResourceQuotaList is a collection of ClusterResourceQuotas
type ClusterResourceQuotaList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty"`

	// Items is a list of ClusterResourceQuotas
	Items []ClusterResourceQuota `json:"items"`
}

// ResourceQuotasStatusByNamespace bundles multiple ResourceQuotaStatusByNamespace
type ResourceQuotasStatusByNamespace []ResourceQuotaStatusByNamespace

// ResourceQuotaStatusByNamespace gives status for a particular namespace
type ResourceQuotaStatusByNamespace struct {
	// Namespace the namespace this status applies to
	Namespace string `json:"namespace"`

	// Status indicates how many resources have been consumed by this namespace
	Status kapi.ResourceQuotaStatus `json:"status"`
}
