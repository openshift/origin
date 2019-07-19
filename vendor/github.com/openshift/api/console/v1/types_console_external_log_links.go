package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConsoleExternalLogLink is an extension for customizing OpenShift web console log links.
type ConsoleExternalLogLink struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ConsoleExternalLogLinkSpec `json:"spec"`
}

// ConsoleExternalLogLinkSpec is the desired log link configuration.
// The log link will appear on the logs tab of the pod details page.
type ConsoleExternalLogLinkSpec struct {
	// text is the display text for the link
	Text string `json:"text"`
	// hrefTemplate is an absolute secure URL (must use https) for the log link including
	// variables to be replaced. Variables are specified in the URL with the format ${variableName},
	// for instance, ${containerName} and will be replaced with the corresponding values
	// from the resource. Resource is a pod.
	// Supported variables are:
	// - ${resourceName} - name of the resource which containes the logs
	// - ${resourceUID} - UID of the resource which contains the logs
	//               - e.g. `11111111-2222-3333-4444-555555555555`
	// - ${containerName} - name of the resource's container that contains the logs
	// - ${resourceNamespace} - namespace of the resource that contains the logs
	// - ${podLabels} - JSON representation of labels matching the pod with the logs
	//             - e.g. `{"key1":"value1","key2":"value2"}`
	//
	// e.g., https://example.com/logs?resourceName=${resourceName}&containerName=${containerName}&resourceNamespace=${resourceNamespace}&podLabels=${podLabels}
	HrefTemplate string `json:"hrefTemplate"`
	// namespaceFilter is a regular expression used to restrict a log link to a
	// matching set of namespaces (e.g., `/^openshift-/g`). If not specified, links will
	// be displayed for all the namespaces.
	// + optional
	NamespaceFilter string `json:"namespaceFilter,omitempty"`
}
