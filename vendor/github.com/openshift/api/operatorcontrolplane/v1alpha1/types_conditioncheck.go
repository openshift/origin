// Package v1alpha1 is an API version in the controlplane.operator.openshift.io group
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodNetworkConnectivityCheck
// +kubebuilder:subresource:status
type PodNetworkConnectivityCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec defines the source and target of the connectivity check
	// +kubebuilder:validation:Required
	// +required
	Spec PodNetworkConnectivityCheckSpec `json:"spec"`

	// Status contains the observed status of the connectivity check
	// +optional
	Status PodNetworkConnectivityCheckStatus `json:"status,omitempty"`
}

type PodNetworkConnectivityCheckSpec struct {
	// SourcePod names the pod from which the condition will be checked
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +required
	SourcePod string `json:"sourcePod"`

	// EndpointAddress to check. A TCP address of the form host:port. Note that
	// if host is a DNS name, then the check would fail if the DNS name cannot
	// be resolved. Specify an IP address for host to bypass DNS name lookup.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^\S+:\d*$`
	// +required
	TargetEndpoint string `json:"targetEndpoint"`
}

// +k8s:deepcopy-gen=true
type PodNetworkConnectivityCheckStatus struct {
	// Successes contains logs successful check actions
	// +optional
	Successes []LogEntry `json:"successes,omitempty"`

	// Failures contains logs of unsuccessful check actions
	// +optional
	Failures []LogEntry `json:"failures,omitempty"`

	// Outages contains logs of time periods of outages
	// +optional
	Outages []OutageEntry `json:"outages,omitempty"`

	// Conditions summarize the status of the check
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []PodNetworkConnectivityCheckCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// LogEntry records events
type LogEntry struct {
	// Start time of check action.
	// +kubebuilder:validation:Required
	// +required
	// +nullable
	Start metav1.Time `json:"time"`

	// Success indicates if the log entry indicates a success or failure.
	// +kubebuilder:validation:Required
	// +required
	Success bool `json:"success"`

	// Reason for status in a machine readable format.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message explaining status in a human readable format.
	// +optional
	Message string `json:"message,omitempty"`

	// Latency records how long the action mentioned in the entry took.
	// +optional
	// +nullable
	Latency metav1.Duration `json:"latency,omitempty"`
}

// OutageEntry records time period of an outage
type OutageEntry struct {

	// Start of outage detected
	// +kubebuilder:validation:Required
	// +required
	// +nullable
	Start metav1.Time `json:"start"`

	// End of outage detected
	// +optional
	// +nullable
	End metav1.Time `json:"end,omitempty"`
}

// PodNetworkConnectivityCheckCondition represents the overall status of the pod network connectivity.
// +k8s:deepcopy-gen=true
type PodNetworkConnectivityCheckCondition struct {

	// Type of the condition
	// +kubebuilder:validation:Required
	// +required
	Type PodNetworkConnectivityCheckConditionType `json:"type"`

	// Status of the condition
	// +kubebuilder:validation:Required
	// +required
	Status metav1.ConditionStatus `json:"status"`

	// Reason for the condition's last status transition in a machine readable format.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message indicating details about last transition in a human readable format.
	// +optional
	Message string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
	// +kubebuilder:validation:Required
	// +required
	// +nullable
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

const (
	LogEntryReasonDNSResolve      = "DNSResolve"
	LogEntryReasonDNSError        = "DNSError"
	LogEntryReasonTCPConnect      = "TCPConnect"
	LogEntryReasonTCPConnectError = "TCPConnectError"
)

type PodNetworkConnectivityCheckConditionType string

const (
	// Reachable indicates that the endpoint was reachable from the pod.
	Reachable PodNetworkConnectivityCheckConditionType = "Reachable"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodNetworkConnectivityCheckList is a collection of PodNetworkConnectivityCheck
type PodNetworkConnectivityCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// Items contains the items
	Items []PodNetworkConnectivityCheck `json:"items"`
}
