package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TestExtensionAdmission controls which ImageStreams are permitted to provide extension test binaries
type TestExtensionAdmission struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestExtensionAdmissionSpec   `json:"spec"`
	Status TestExtensionAdmissionStatus `json:"status,omitempty"`
}

// TestExtensionAdmissionSpec defines the desired state of TestExtensionAdmission
type TestExtensionAdmissionSpec struct {
	// Permit is a list of permitted ImageStream patterns in format "namespace/imagestream".
	// Supports wildcards like "openshift/*", "*/*", or specific "namespace/stream"
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Permit []string `json:"permit"`
}

// TestExtensionAdmissionStatus defines the observed state of TestExtensionAdmission
type TestExtensionAdmissionStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
