package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Infrastructure holds cluster-wide information about Infrastructure.  The canonical name is `cluster`
type Infrastructure struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec InfrastructureSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status InfrastructureStatus `json:"status"`
}

// InfrastructureSpec contains settings that apply to the cluster infrastructure.
type InfrastructureSpec struct {
	// secret reference?
	// configmap reference to file?
}

// InfrastructureStatus describes the infrastructure the cluster is leveraging.
type InfrastructureStatus struct {
	// platform is the underlying infrastructure provider for the cluster. This
	// value controls whether infrastructure automation such as service load
	// balancers, dynamic volume provisioning, machine creation and deletion, and
	// other integrations are enabled. If None, no infrastructure automation is
	// enabled. Allowed values are "AWS", "Azure", "GCP", "Libvirt",
	// "OpenStack", "VSphere", and "None". Individual components may not support
	// all platforms, and must handle unrecognized platforms as None if they do
	// not support that platform.
	Platform PlatformType `json:"platform,omitempty"`

	// etcdDiscoveryDomain is the domain used to fetch the SRV records for discovering
	// etcd servers and clients.
	// For more info: https://github.com/etcd-io/etcd/blob/329be66e8b3f9e2e6af83c123ff89297e49ebd15/Documentation/op-guide/clustering.md#dns-discovery
	EtcdDiscoveryDomain string `json:"etcdDiscoveryDomain"`

	// apiServerURL is a valid URL with scheme(http/https), address and port.
	// apiServerURL can be used by components like kubelet on machines, to contact the `apisever`
	// using the infrastructure provider rather than the kubernetes networking.
	APIServerURL string `json:"apiServerURL"`
}

// PlatformType is a specific supported infrastructure provider.
type PlatformType string

const (
	// AWSPlatform represents Amazon Web Services infrastructure.
	AWSPlatform PlatformType = "AWS"

	// AzurePlatform represents Microsoft Azure infrastructure.
	AzurePlatform PlatformType = "Azure"

	// GCPPlatform represents Google Cloud Platform infrastructure.
	GCPPlatform PlatformType = "GCP"

	// LibvirtPlatform represents libvirt infrastructure.
	LibvirtPlatform PlatformType = "Libvirt"

	// OpenStackPlatform represents OpenStack infrastructure.
	OpenStackPlatform PlatformType = "OpenStack"

	// NonePlatform means there is no infrastructure provider.
	NonePlatform PlatformType = "None"

	// VSpherePlatform represents VMWare vSphere infrastructure.
	VSpherePlatform PlatformType = "VSphere"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureList is
type InfrastructureList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Infrastructure `json:"items"`
}
