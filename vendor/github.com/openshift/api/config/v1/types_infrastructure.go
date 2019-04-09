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
	// +required
	Spec InfrastructureSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status InfrastructureStatus `json:"status"`
}

// InfrastructureSpec contains settings that apply to the cluster infrastructure.
type InfrastructureSpec struct {
	// cloudConfig is a reference to a ConfigMap containing the cloud provider configuration file.
	// This configuration file is used to configure the Kubernetes cloud provider integration
	// when using the built-in cloud provider integration or the external cloud controller manager.
	// The namespace for this config map is openshift-config.
	// +optional
	CloudConfig ConfigMapFileReference `json:"cloudConfig"`
}

// InfrastructureStatus describes the infrastructure the cluster is leveraging.
type InfrastructureStatus struct {
	// infrastructureName uniquely identifies a cluster with a human friendly name.
	// Once set it should not be changed. Must be of max length 27 and must have only
	// alphanumeric or hyphen characters.
	InfrastructureName string `json:"infrastructureName"`

	// platform is the underlying infrastructure provider for the cluster. This
	// value controls whether infrastructure automation such as service load
	// balancers, dynamic volume provisioning, machine creation and deletion, and
	// other integrations are enabled. If None, no infrastructure automation is
	// enabled. Allowed values are "AWS", "Azure", "BareMetal", "GCP", "Libvirt",
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
	// AWSPlatformType represents Amazon Web Services infrastructure.
	AWSPlatformType PlatformType = "AWS"

	// AzurePlatformType represents Microsoft Azure infrastructure.
	AzurePlatformType PlatformType = "Azure"

	// BareMetalPlatformType represents managed bare metal infrastructure.
	BareMetalPlatformType PlatformType = "BareMetal"

	// GCPPlatformType represents Google Cloud Platform infrastructure.
	GCPPlatformType PlatformType = "GCP"

	// LibvirtPlatformType represents libvirt infrastructure.
	LibvirtPlatformType PlatformType = "Libvirt"

	// OpenStackPlatformType represents OpenStack infrastructure.
	OpenStackPlatformType PlatformType = "OpenStack"

	// NonePlatformType means there is no infrastructure provider.
	NonePlatformType PlatformType = "None"

	// VSpherePlatformType represents VMWare vSphere infrastructure.
	VSpherePlatformType PlatformType = "VSphere"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureList is
type InfrastructureList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []Infrastructure `json:"items"`
}
