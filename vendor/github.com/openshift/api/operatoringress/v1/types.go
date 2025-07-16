package v1

import (
	configv1 "github.com/openshift/api/config/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=dnsrecords,scope=Namespaced
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/584
// +openshift:capability=Ingress
// +openshift:file-pattern=cvoRunLevel=0000_50,operatorName=dns,operatorOrdering=01

// DNSRecord is a DNS record managed in the zones defined by
// dns.config.openshift.io/cluster .spec.publicZone and .spec.privateZone.
//
// Cluster admin manipulation of this resource is not supported. This resource
// is only for internal communication of OpenShift operators.
//
// If DNSManagementPolicy is "Unmanaged", the operator will not be responsible
// for managing the DNS records on the cloud provider.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type DNSRecord struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired behavior of the dnsRecord.
	Spec DNSRecordSpec `json:"spec"`
	// status is the most recently observed status of the dnsRecord.
	Status DNSRecordStatus `json:"status"`
}

// DNSRecordSpec contains the details of a DNS record.
type DNSRecordSpec struct {
	// dnsName is the hostname of the DNS record
	//
	// +kubebuilder:validation:MinLength=1
	// +required
	DNSName string `json:"dnsName"`
	// targets are record targets.
	//
	// +kubebuilder:validation:MinItems=1
	// +required
	Targets []string `json:"targets"`
	// recordType is the DNS record type. For example, "A" or "CNAME".
	// +required
	RecordType DNSRecordType `json:"recordType"`
	// recordTTL is the record TTL in seconds. If zero, the default is 30.
	// RecordTTL will not be used in AWS regions Alias targets, but
	// will be used in CNAME targets, per AWS API contract.
	//
	// +kubebuilder:validation:Minimum=0
	// +required
	RecordTTL int64 `json:"recordTTL"`
	// dnsManagementPolicy denotes the current policy applied on the DNS
	// record. Records that have policy set as "Unmanaged" are ignored by
	// the ingress operator.  This means that the DNS record on the cloud
	// provider is not managed by the operator, and the "Published" status
	// condition will be updated to "Unknown" status, since it is externally
	// managed. Any existing record on the cloud provider can be deleted at
	// the discretion of the cluster admin.
	//
	// This field defaults to Managed. Valid values are "Managed" and
	// "Unmanaged".
	//
	// +kubebuilder:default:="Managed"
	// +required
	// +default="Managed"
	DNSManagementPolicy DNSManagementPolicy `json:"dnsManagementPolicy,omitempty"`
}

// DNSRecordStatus is the most recently observed status of each record.
type DNSRecordStatus struct {
	// zones are the status of the record in each zone.
	// +optional
	Zones []DNSZoneStatus `json:"zones,omitempty"`

	// observedGeneration is the most recently observed generation of the
	// DNSRecord.  When the DNSRecord is updated, the controller updates the
	// corresponding record in each managed zone.  If an update for a
	// particular zone fails, that failure is recorded in the status
	// condition for the zone so that the controller can determine that it
	// needs to retry the update for that specific zone.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// DNSZoneStatus is the status of a record within a specific zone.
type DNSZoneStatus struct {
	// dnsZone is the zone where the record is published.
	DNSZone configv1.DNSZone `json:"dnsZone"`
	// conditions are any conditions associated with the record in the zone.
	//
	// If publishing the record succeeds, the "Published" condition will be
	// set with status "True" and upon failure it will be set to "False" along
	// with the reason and message describing the cause of the failure.
	Conditions []DNSZoneCondition `json:"conditions,omitempty"`
}

var (
	// Failed means the record is not available within a zone.
	// DEPRECATED: will be removed soon, use DNSRecordPublishedConditionType.
	DNSRecordFailedConditionType = "Failed"

	// Published means the record is published to a zone.
	DNSRecordPublishedConditionType = "Published"
)

// DNSZoneCondition is just the standard condition fields.
type DNSZoneCondition struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	Type string `json:"type"`
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
	// CNAMERecordType is an RFC 1035 CNAME record.
	CNAMERecordType DNSRecordType = "CNAME"

	// ARecordType is an RFC 1035 A record.
	ARecordType DNSRecordType = "A"
)

// DNSManagementPolicy is a policy for configuring how the dns controller
// manages DNSRecords.
//
// +kubebuilder:validation:Enum=Managed;Unmanaged
type DNSManagementPolicy string

const (
	// ManagedDNS configures the dns controller to manage the lifecycle of the
	// DNS record on the cloud platform.
	ManagedDNS DNSManagementPolicy = "Managed"
	// UnmanagedDNS configures the dns controller not to create a DNS record or
	// manage any existing DNS record and allows the DNS record on the cloud
	// provider to be managed by the cluster admin.
	UnmanagedDNS DNSManagementPolicy = "Unmanaged"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DNSRecordList contains a list of dnsrecords.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type DNSRecordList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []DNSRecord `json:"items"`
}
