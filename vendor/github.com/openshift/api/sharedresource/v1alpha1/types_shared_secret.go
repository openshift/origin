package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SharedSecret allows a Secret to be shared across namespaces.
// Pods can mount the shared Secret by adding a CSI volume to the pod specification using the
// "csi.sharedresource.openshift.io" CSI driver and a reference to the SharedSecret in the volume attributes:
//
// spec:
//
//	volumes:
//	- name: shared-secret
//	  csi:
//	    driver: csi.sharedresource.openshift.io
//	    volumeAttributes:
//	      sharedSecret: my-share
//
// For the mount to be successful, the pod's service account must be granted permission to 'use' the named SharedSecret object
// within its namespace with an appropriate Role and RoleBinding. For compactness, here are example `oc` invocations for creating
// such Role and RoleBinding objects.
//
//	`oc create role shared-resource-my-share --verb=use --resource=sharedsecrets.sharedresource.openshift.io --resource-name=my-share`
//	`oc create rolebinding shared-resource-my-share --role=shared-resource-my-share --serviceaccount=my-namespace:default`
//
// Shared resource objects, in this case Secrets, have default permissions of list, get, and watch for system authenticated users.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=sharedsecrets,scope=Cluster
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/979
// +kubebuilder:metadata:annotations="description=Extension for sharing Secrets across Namespaces"
// +kubebuilder:metadata:annotations="displayName=SharedSecret"
type SharedSecret struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired shared secret
	// +required
	Spec SharedSecretSpec `json:"spec,omitempty"`

	// status is the observed status of the shared secret
	Status SharedSecretStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SharedSecretList contains a list of SharedSecret objects.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type SharedSecretList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SharedSecret `json:"items"`
}

// SharedSecretReference contains information about which Secret to share
type SharedSecretReference struct {
	// name represents the name of the Secret that is being referenced.
	// +required
	Name string `json:"name"`
	// namespace represents the namespace where the referenced Secret is located.
	// +required
	Namespace string `json:"namespace"`
}

// SharedSecretSpec defines the desired state of a SharedSecret
// +k8s:openapi-gen=true
type SharedSecretSpec struct {
	// secretRef is a reference to the Secret to share
	// +required
	SecretRef SharedSecretReference `json:"secretRef"`
	// description is a user readable explanation of what the backing resource provides.
	Description string `json:"description,omitempty"`
}

// SharedSecretStatus contains the observed status of the shared resource
type SharedSecretStatus struct {
	// conditions represents any observations made on this particular shared resource by the underlying CSI driver or Share controller.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}
