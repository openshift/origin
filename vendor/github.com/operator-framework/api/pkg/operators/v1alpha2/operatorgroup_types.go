package v1alpha2

import (
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	OperatorGroupAnnotationKey             = "olm.operatorGroup"
	OperatorGroupNamespaceAnnotationKey    = "olm.operatorNamespace"
	OperatorGroupTargetsAnnotationKey      = "olm.targetNamespaces"
	OperatorGroupProvidedAPIsAnnotationKey = "olm.providedAPIs"

	OperatorGroupKind = "OperatorGroup"
)

// OperatorGroupSpec is the spec for an OperatorGroup resource.
type OperatorGroupSpec struct {
	// Selector selects the OperatorGroup's target namespaces.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// TargetNamespaces is an explicit set of namespaces to target.
	// If it is set, Selector is ignored.
	// +optional
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`

	// ServiceAccountName is the admin specified service account which will be
	// used to deploy operator(s) in this operator group.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Static tells OLM not to update the OperatorGroup's providedAPIs annotation
	// +optional
	StaticProvidedAPIs bool `json:"staticProvidedAPIs,omitempty"`
}

// OperatorGroupStatus is the status for an OperatorGroupResource.
type OperatorGroupStatus struct {
	// Namespaces is the set of target namespaces for the OperatorGroup.
	Namespaces []string `json:"namespaces,omitempty"`

	// ServiceAccountRef references the service account object specified.
	ServiceAccountRef *corev1.ObjectReference `json:"serviceAccountRef,omitempty"`

	// LastUpdated is a timestamp of the last time the OperatorGroup's status was Updated.
	LastUpdated *metav1.Time `json:"lastUpdated"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +kubebuilder:resource:shortName=og,categories=olm
// +kubebuilder:subresource:status

// OperatorGroup is the unit of multitenancy for OLM managed operators.
// It constrains the installation of operators in its namespace to a specified set of target namespaces.
type OperatorGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// +optional
	Spec   OperatorGroupSpec   `json:"spec"`
	Status OperatorGroupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperatorGroupList is a list of OperatorGroup resources.
type OperatorGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []OperatorGroup `json:"items"`
}

func (o *OperatorGroup) BuildTargetNamespaces() string {
	sort.Strings(o.Status.Namespaces)
	return strings.Join(o.Status.Namespaces, ",")
}

// IsServiceAccountSpecified returns true if the spec has a service account name specified.
func (o *OperatorGroup) IsServiceAccountSpecified() bool {
	if o.Spec.ServiceAccountName == "" {
		return false
	}

	return true
}

// HasServiceAccountSynced returns true if the service account specified has been synced.
func (o *OperatorGroup) HasServiceAccountSynced() bool {
	if o.IsServiceAccountSpecified() && o.Status.ServiceAccountRef != nil {
		return true
	}

	return false
}
