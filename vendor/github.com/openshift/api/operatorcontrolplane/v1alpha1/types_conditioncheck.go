// Package v1alpha1 is an API version in the controlplane.operator.openshift.io group
package v1alpha1

import (
	v1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodNetworkConnectivityCheck
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=podnetworkconnectivitychecks,scope=Namespaced
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/639
// +openshift:file-pattern=cvoRunLevel=0000_10,operatorName=network,operatorOrdering=01
// +kubebuilder:metadata:annotations=include.release.openshift.io/self-managed-high-availability=true
// +openshift:compatibility-gen:level=4
type PodNetworkConnectivityCheck struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata"`

	// spec defines the source and target of the connectivity check
	// +required
	Spec PodNetworkConnectivityCheckSpec `json:"spec"`

	// status contains the observed status of the connectivity check
	// +optional
	Status PodNetworkConnectivityCheckStatus `json:"status,omitempty"`
}

type PodNetworkConnectivityCheckSpec struct {
	// sourcePod names the pod from which the condition will be checked
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +required
	SourcePod string `json:"sourcePod"`

	// EndpointAddress to check. A TCP address of the form host:port. Note that
	// if host is a DNS name, then the check would fail if the DNS name cannot
	// be resolved. Specify an IP address for host to bypass DNS name lookup.
	// +kubebuilder:validation:Pattern=`^\S+:\d*$`
	// +required
	TargetEndpoint string `json:"targetEndpoint"`

	// TLSClientCert, if specified, references a kubernetes.io/tls type secret with 'tls.crt' and
	// 'tls.key' entries containing an optional TLS client certificate and key to be used when
	// checking endpoints that require a client certificate in order to gracefully preform the
	// scan without causing excessive logging in the endpoint process. The secret must exist in
	// the same namespace as this resource.
	// +optional
	TLSClientCert v1.SecretNameReference `json:"tlsClientCert,omitempty"`
}

// +k8s:deepcopy-gen=true
type PodNetworkConnectivityCheckStatus struct {
	// successes contains logs successful check actions
	// +optional
	Successes []LogEntry `json:"successes,omitempty"`

	// failures contains logs of unsuccessful check actions
	// +optional
	Failures []LogEntry `json:"failures,omitempty"`

	// outages contains logs of time periods of outages
	// +optional
	Outages []OutageEntry `json:"outages,omitempty"`

	// conditions summarize the status of the check
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []PodNetworkConnectivityCheckCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// LogEntry records events
type LogEntry struct {
	// Start time of check action.
	// +required
	// +nullable
	Start metav1.Time `json:"time"`

	// success indicates if the log entry indicates a success or failure.
	// +required
	Success bool `json:"success"`

	// reason for status in a machine readable format.
	// +optional
	Reason string `json:"reason,omitempty"`

	// message explaining status in a human readable format.
	// +optional
	Message string `json:"message,omitempty"`

	// latency records how long the action mentioned in the entry took.
	// +optional
	// +nullable
	Latency metav1.Duration `json:"latency,omitempty"`
}

// OutageEntry records time period of an outage
type OutageEntry struct {

	// start of outage detected
	// +required
	// +nullable
	Start metav1.Time `json:"start"`

	// end of outage detected
	// +optional
	// +nullable
	End metav1.Time `json:"end,omitempty"`

	// startLogs contains log entries related to the start of this outage. Should contain
	// the original failure, any entries where the failure mode changed.
	// +optional
	StartLogs []LogEntry `json:"startLogs,omitempty"`

	// endLogs contains log entries related to the end of this outage. Should contain the success
	// entry that resolved the outage and possibly a few of the failure log entries that preceded it.
	// +optional
	EndLogs []LogEntry `json:"endLogs,omitempty"`

	// message summarizes outage details in a human readable format.
	// +optional
	Message string `json:"message,omitempty"`
}

// PodNetworkConnectivityCheckCondition represents the overall status of the pod network connectivity.
// +k8s:deepcopy-gen=true
type PodNetworkConnectivityCheckCondition struct {

	// type of the condition
	// +required
	Type PodNetworkConnectivityCheckConditionType `json:"type"`

	// status of the condition
	// +required
	Status metav1.ConditionStatus `json:"status"`

	// reason for the condition's last status transition in a machine readable format.
	// +optional
	Reason string `json:"reason,omitempty"`

	// message indicating details about last transition in a human readable format.
	// +optional
	Message string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
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
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type PodNetworkConnectivityCheckList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	// items contains the items
	Items []PodNetworkConnectivityCheck `json:"items"`
}
