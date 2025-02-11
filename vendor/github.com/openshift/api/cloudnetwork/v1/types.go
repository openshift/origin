package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CloudPrivateIPConfig performs an assignment of a private IP address to the
// primary NIC associated with cloud VMs. This is done by specifying the IP and
// Kubernetes node which the IP should be assigned to. This CRD is intended to
// be used by the network plugin which manages the cluster network. The spec
// side represents the desired state requested by the network plugin, and the
// status side represents the current state that this CRD's controller has
// executed. No users will have permission to modify it, and if a cluster-admin
// decides to edit it for some reason, their changes will be overwritten the
// next time the network plugin reconciles the object. Note: the CR's name
// must specify the requested private IP address (can be IPv4 or IPv6).
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=cloudprivateipconfigs,scope=Cluster
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/859
// +openshift:file-pattern=operatorOrdering=001
// +openshift:compatibility-gen:level=1
type CloudPrivateIPConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// spec is the definition of the desired private IP request.
	// +required
	Spec CloudPrivateIPConfigSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	// status is the observed status of the desired private IP request. Read-only.
	// +optional
	Status CloudPrivateIPConfigStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// CloudPrivateIPConfigSpec consists of a node name which the private IP should be assigned to.
// +k8s:openapi-gen=true
type CloudPrivateIPConfigSpec struct {
	// node is the node name, as specified by the Kubernetes field: node.metadata.name
	// +optional
	Node string `json:"node" protobuf:"bytes,1,opt,name=node"`
}

// CloudPrivateIPConfigStatus specifies the node assignment together with its assignment condition.
// +k8s:openapi-gen=true
type CloudPrivateIPConfigStatus struct {
	// node is the node name, as specified by the Kubernetes field: node.metadata.name
	// +optional
	Node string `json:"node" protobuf:"bytes,1,opt,name=node"`
	// condition is the assignment condition of the private IP and its status
	// +required
	Conditions []metav1.Condition `json:"conditions" protobuf:"bytes,2,rep,name=conditions"`
}

// CloudPrivateIPConfigConditionType specifies the current condition type of the CloudPrivateIPConfig
type CloudPrivateIPConfigConditionType string

const (
	// Assigned is the condition type of the cloud private IP request.
	// It is paired with the following ConditionStatus:
	// - True - in the case of a successful assignment
	// - False - in the case of a failed assignment
	// - Unknown - in the case of a pending assignment
	Assigned CloudPrivateIPConfigConditionType = "Assigned"
)

// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=cloudprivateipconfig
// CloudPrivateIPConfigList is the list of CloudPrivateIPConfigList.
// +openshift:compatibility-gen:level=1
type CloudPrivateIPConfigList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// List of CloudPrivateIPConfig.
	Items []CloudPrivateIPConfig `json:"items" protobuf:"bytes,2,rep,name=items"`
}
