package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConsoleYAMLSample is an extension for customizing OpenShift web console YAML samples.
//
// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type ConsoleYAMLSample struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata"`

	Spec ConsoleYAMLSampleSpec `json:"spec"`
}

// ConsoleYAMLSampleSpec is the desired YAML sample configuration.
// Samples will appear with their descriptions in a samples sidebar
// when creating a resources in the web console.
type ConsoleYAMLSampleSpec struct {
	// targetResource contains apiVersion and kind of the resource
	// YAML sample is representating.
	TargetResource metav1.TypeMeta `json:"targetResource"`
	// title of the YAML sample.
	Title ConsoleYAMLSampleTitle `json:"title"`
	// description of the YAML sample.
	Description ConsoleYAMLSampleDescription `json:"description"`
	// yaml is the YAML sample to display.
	YAML ConsoleYAMLSampleYAML `json:"yaml"`
	// snippet indicates that the YAML sample is not the full YAML resource
	// definition, but a fragment that can be inserted into the existing
	// YAML document at the user's cursor.
	// +optional
	Snippet bool `json:"snippet"`
}

// ConsoleYAMLSampleTitle of the YAML sample.
// +kubebuilder:validation:Pattern=`^(.|\s)*\S(.|\s)*$`
type ConsoleYAMLSampleTitle string

// ConsoleYAMLSampleDescription of the YAML sample.
// +kubebuilder:validation:Pattern=`^(.|\s)*\S(.|\s)*$`
type ConsoleYAMLSampleDescription string

// ConsoleYAMLSampleYAML is the YAML sample to display.
// +kubebuilder:validation:Pattern=`^(.|\s)*\S(.|\s)*$`
type ConsoleYAMLSampleYAML string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Compatibility level 2: Stable within a major release for a minimum of 9 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=2
type ConsoleYAMLSampleList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []ConsoleYAMLSample `json:"items"`
}
