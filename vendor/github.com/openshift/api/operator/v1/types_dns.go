package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DNS manages the CoreDNS component to provide a name resolution service
// for pods and services in the cluster.
//
// This supports the DNS-based service discovery specification:
// https://github.com/kubernetes/dns/blob/master/docs/specification.md
//
// More details: https://kubernetes.io/docs/tasks/administer-cluster/coredns
type DNS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired behavior of the DNS.
	Spec DNSSpec `json:"spec,omitempty"`
	// status is the most recently observed status of the DNS.
	Status DNSStatus `json:"status,omitempty"`
}

// DNSSpec is the specification of the desired behavior of the DNS.
type DNSSpec struct {
}

// DNSStatus defines the observed status of the DNS.
type DNSStatus struct {
	// clusterIP is the service IP through which this DNS is made available.
	//
	// In the case of the default DNS, this will be a well known IP that is used
	// as the default nameserver for pods that are using the default ClusterFirst DNS policy.
	//
	// In general, this IP can be specified in a pod's spec.dnsConfig.nameservers list
	// or used explicitly when performing name resolution from within the cluster.
	// Example: dig foo.com @<service IP>
	//
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
	ClusterIP string `json:"clusterIP"`

	// clusterDomain is the local cluster DNS domain suffix for DNS services.
	// This will be a subdomain as defined in RFC 1034,
	// section 3.5: https://tools.ietf.org/html/rfc1034#section-3.5
	// Example: "cluster.local"
	//
	// More info: https://kubernetes.io/docs/concepts/services-networking/dns-pod-service
	ClusterDomain string `json:"clusterDomain"`

	// conditions represent the latest available observations of a
	// DNS's current state.
	//
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []DNSCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// DNSCondition contains details for the current condition of this DNS.
type DNSCondition struct {
	// type is the type of the condition.
	//
	// These are the supported DNS condition types:
	//
	//   * Available
	//   - True if the following conditions are met:
	//     * DNS controller daemonset is available.
	//   - False if any of those conditions are unsatisfied.
	Type DNSConditionType `json:"type"`

	// status is the status of the condition. Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`

	// lastTransitionTime is the last time a condition status transitioned from
	// one state to another.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime"`

	// reason is a brief machine readable explanation for the condition's last
	// transition.
	Reason string `json:"reason,omitempty"`

	// message is a human readable description of the details of the last
	// transition, complementing reason.
	Message string `json:"message,omitempty"`
}

// DNSConditionType is a valid value for DNSCondition
type DNSConditionType string

const (
	// Available indicates the DNS controller daemonset is available.
	DNSControllerAvailable DNSConditionType = "Available"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DNSList contains a list of DNS
type DNSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []DNS `json:"items"`
}
