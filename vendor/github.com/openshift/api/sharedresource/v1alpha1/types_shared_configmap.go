package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SharedConfigMap allows a ConfigMap to be shared across namespaces.
// Pods can mount the shared ConfigMap by adding a CSI volume to the pod specification using the
// "csi.sharedresource.openshift.io" CSI driver and a reference to the SharedConfigMap in the volume attributes:
//
// spec:
//
//	volumes:
//	- name: shared-configmap
//	  csi:
//	    driver: csi.sharedresource.openshift.io
//	    volumeAttributes:
//	      sharedConfigMap: my-share
//
// For the mount to be successful, the pod's service account must be granted permission to 'use' the named SharedConfigMap object
// within its namespace with an appropriate Role and RoleBinding. For compactness, here are example `oc` invocations for creating
// such Role and RoleBinding objects.
//
//	`oc create role shared-resource-my-share --verb=use --resource=sharedconfigmaps.sharedresource.openshift.io --resource-name=my-share`
//	`oc create rolebinding shared-resource-my-share --role=shared-resource-my-share --serviceaccount=my-namespace:default`
//
// Shared resource objects, in this case ConfigMaps, have default permissions of list, get, and watch for system authenticated users.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// These capabilities should not be used by applications needing long term support.
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=sharedconfigmaps,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/979
// +kubebuilder:metadata:annotations="description=Extension for sharing ConfigMaps across Namespaces"
// +kubebuilder:metadata:annotations="displayName=SharedConfigMap"
// +k8s:openapi-gen=true
// +openshift:compatibility-gen:level=4
type SharedConfigMap struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired shared configmap
	// +required
	Spec SharedConfigMapSpec `json:"spec,omitempty"`

	// status is the observed status of the shared configmap
	Status SharedConfigMapStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SharedConfigMapList contains a list of SharedConfigMap objects.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type SharedConfigMapList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SharedConfigMap `json:"items"`
}

// SharedConfigMapReference contains information about which ConfigMap to share
type SharedConfigMapReference struct {
	// name represents the name of the ConfigMap that is being referenced.
	// +required
	Name string `json:"name"`
	// namespace represents the namespace where the referenced ConfigMap is located.
	// +required
	Namespace string `json:"namespace"`
}

// SharedConfigMapSpec defines the desired state of a SharedConfigMap
// +k8s:openapi-gen=true
type SharedConfigMapSpec struct {
	//configMapRef is a reference to the ConfigMap to share
	// +required
	ConfigMapRef SharedConfigMapReference `json:"configMapRef"`
	// description is a user readable explanation of what the backing resource provides.
	Description string `json:"description,omitempty"`
}

// SharedSecretStatus contains the observed status of the shared resource
type SharedConfigMapStatus struct {
	// conditions represents any observations made on this particular shared resource by the underlying CSI driver or Share controller.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
