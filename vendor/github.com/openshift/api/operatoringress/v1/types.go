package v1

import (
	configv1 "github.com/openshift/api/config/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DNSRecord is a DNS record managed in the zones defined by
// dns.config.openshift.io/cluster .spec.publicZone and .spec.privateZone.
//
// Cluster admin manipulation of this resource is not supported. This resource
// is only for internal communication of OpenShift operators.
type DNSRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired behavior of the dnsecord.
	Spec DNSRecordSpec `json:"spec"`
	// status is the most recently observed status of the dnsRecord.
	Status DNSRecordStatus `json:"status"`
}

// DNSRecordSpec contains the details of a DNS record.
type DNSRecordSpec struct {
	// dnsName is the hostname of the DNS record
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +required
	DNSName string `json:"dnsName"`
	// targets are record targets.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +required
	Targets []string `json:"targets"`
	// recordType is the DNS record type. For example, "A" or "CNAME".
	// +kubebuilder:validation:Required
	// +required
	RecordType DNSRecordType `json:"recordType"`
	// recordTTL is the record TTL in seconds. If zero, the default is 30.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +required
	RecordTTL int64 `json:"recordTTL"`
}

// DNSRecordStatus is the most recently observed status of each record.
type DNSRecordStatus struct {
	// zones are the status of the record in each zone.
	Zones []DNSZoneStatus `json:"zones,omitempty"`
}

// DNSZoneStatus is the status of a record within a specific zone.
type DNSZoneStatus struct {
	// dnsZone is the zone where the record is published.
	DNSZone configv1.DNSZone `json:"dnsZone"`
	// conditions are any conditions associated with the record in the zone.
	//
	// If publishing the record fails, the "Failed" condition will be set with a
	// reason and message describing the cause of the failure.
	Conditions []DNSZoneCondition `json:"conditions,omitempty"`
}

var (
	// Failed means the record is not available within a zone.
	DNSRecordFailedConditionType = "Failed"
)

// DNSZoneCondition is just the standard condition fields.
type DNSZoneCondition struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +required
	Type string `json:"type"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +required
	Status             string      `json:"status"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	Reason             string      `json:"reason,omitempty"`
	Message            string      `json:"message,omitempty"`
}

// DNSRecordType is a DNS resource record type.
// +kubebuilder:validation:Enum=CNAME;A
type DNSRecordType string

const (
	// CNAME is an RFC 1035 CNAME record.
	CNAMERecordType DNSRecordType = "CNAME"

	// CNAME is an RFC 1035 A record.
	ARecordType DNSRecordType = "A"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// DNSRecordList contains a list of dnsrecords.
type DNSRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSRecord `json:"items"`
}
