package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

// +genclient=true
// +nonNamespaced=true

// ClusterResourceQuota mirrors ResourceQuota at a cluster scope.  This object is easily convertible to
// synthetic ResourceQuota object to allow quota evaluation re-use.
type ClusterResourceQuota struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired quota
	Spec ClusterResourceQuotaSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`

	// Status defines the actual enforced quota and its current usage
	Status ClusterResourceQuotaStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ClusterResourceQuotaSpec defines the desired quota restrictions
type ClusterResourceQuotaSpec struct {
	// Selector is the selector used to match projects.
	// It should only select active projects on the scale of dozens (though it can select
	// many more less active projects).  These projects will contend on object creation through
	// this resource.
	Selector ClusterResourceQuotaSelector `json:"selector" protobuf:"bytes,1,opt,name=selector"`

	// Quota defines the desired quota
	Quota kapi.ResourceQuotaSpec `json:"quota" protobuf:"bytes,2,opt,name=quota"`
}

// ClusterResourceQuotaSelector is used to select projects.  At least one of LabelSelector or AnnotationSelector
// must present.  If only one is present, it is the only selection criteria.  If both are specified,
// the project must match both restrictions.
type ClusterResourceQuotaSelector struct {
	// LabelSelector is used to select projects by label.
	LabelSelector *metav1.LabelSelector `json:"labels" protobuf:"bytes,1,opt,name=labels"`

	// AnnotationSelector is used to select projects by annotation.
	AnnotationSelector map[string]string `json:"annotations" protobuf:"bytes,2,rep,name=annotations"`
}

// ClusterResourceQuotaStatus defines the actual enforced quota and its current usage
type ClusterResourceQuotaStatus struct {
	// Total defines the actual enforced quota and its current usage across all projects
	Total kapi.ResourceQuotaStatus `json:"total" protobuf:"bytes,1,opt,name=total"`

	// Namespaces slices the usage by project.  This division allows for quick resolution of
	// deletion reconciliation inside of a single project without requiring a recalculation
	// across all projects.  This can be used to pull the deltas for a given project.
	Namespaces ResourceQuotasStatusByNamespace `json:"namespaces" protobuf:"bytes,2,rep,name=namespaces"`
}

// ClusterResourceQuotaList is a collection of ClusterResourceQuotas
type ClusterResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is a list of ClusterResourceQuotas
	Items []ClusterResourceQuota `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ResourceQuotasStatusByNamespace bundles multiple ResourceQuotaStatusByNamespace
type ResourceQuotasStatusByNamespace []ResourceQuotaStatusByNamespace

// ResourceQuotaStatusByNamespace gives status for a particular project
type ResourceQuotaStatusByNamespace struct {
	// Namespace the project this status applies to
	Namespace string `json:"namespace" protobuf:"bytes,1,opt,name=namespace"`

	// Status indicates how many resources have been consumed by this project
	Status kapi.ResourceQuotaStatus `json:"status" protobuf:"bytes,2,opt,name=status"`
}

// AppliedClusterResourceQuota mirrors ClusterResourceQuota at a project scope, for projection
// into a project.  It allows a project-admin to know which ClusterResourceQuotas are applied to
// his project and their associated usage.
type AppliedClusterResourceQuota struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired quota
	Spec ClusterResourceQuotaSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`

	// Status defines the actual enforced quota and its current usage
	Status ClusterResourceQuotaStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// AppliedClusterResourceQuotaList is a collection of AppliedClusterResourceQuotas
type AppliedClusterResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is a list of AppliedClusterResourceQuota
	Items []AppliedClusterResourceQuota `json:"items" protobuf:"bytes,2,rep,name=items"`
}
