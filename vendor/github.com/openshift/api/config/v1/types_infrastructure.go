package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Infrastructure holds cluster-wide information about Infrastructure.  The canonical name is `cluster`
// TODO this object is an example of a possible grouping and is subject to change or removal
type Infrastructure struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec InfrastructureSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status InfrastructureStatus `json:"status"`
}

type InfrastructureSpec struct {
	// secret reference?
	// configmap reference to file?
}

type InfrastructureStatus struct {
	// platform is the underlying infrastructure provider for the cluster. This
	// value controls whether infrastructure automation such as service load
	// balancers, dynamic volume provisioning, machine creation and deletion, and
	// other integrations are enabled. If None, no infrastructure automation is
	// enabled.
	Platform PlatformType `json:"platform,omitempty"`
}

// platformType is a specific supported infrastructure provider.
type PlatformType string

const (
	// awsPlatform represents Amazon AWS.
	AWSPlatform PlatformType = "AWS"

	// openStackPlatform represents OpenStack.
	OpenStackPlatform PlatformType = "OpenStack"

	// libvirtPlatform represents libvirt.
	LibvirtPlatform PlatformType = "Libvirt"

	// nonePlatform means there is no infrastructure provider.
	NonePlatform PlatformType = "None"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InfrastructureList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Infrastructure `json:"items"`
}
