package schema

import (
	"time"
)

type VMRecoveryPointListIntent struct {
	APIVersion *string `json:"api_version"`

	Entities []*VMRecoveryPointIntent `json:"entities,omitempty"`

	Metadata *ListMetadata `json:"metadata"`
}

type VMRecoveryPointIntent struct {

	// api version
	APIVersion *string `json:"api_version,omitempty"`

	// metadata
	// Required: true
	Metadata *Metadata `json:"metadata"`

	// spec
	Spec *VMRecoveryPoint `json:"spec,omitempty"`

	// status
	Status *VMRecoveryPointDefStatus `json:"status,omitempty"`
}

type VMRecoveryPointRequest struct {

	// api version
	APIVersion *string `json:"api_version,omitempty"`

	// metadata
	// Required: true
	Metadata *Metadata `json:"metadata"`

	// spec
	Spec *VMRecoveryPoint `json:"spec"`
}

type VMRecoveryPoint struct {

	// Reference to the availability zone where this recovery point is
	// located
	//
	AvailabilityZoneReference *Reference `json:"availability_zone_reference,omitempty"`

	// Reference to the cluster in the availability zone where this recovery
	// point is located.
	//
	ClusterReference *Reference `json:"cluster_reference,omitempty"`

	// Name of the recovery point.
	// Max Length: 64
	Name string `json:"name,omitempty"`

	// resources
	Resources *VMRecoveryPointResources `json:"resources,omitempty"`
}

type VMRecoveryPointDefStatus struct {

	// Reference to the availability zone where this recovery point is
	// located.
	//
	AvailabilityZoneReference *Reference `json:"availability_zone_reference,omitempty"`

	// Reference to the cluster in the availability zone where this recovery
	// point is located.
	//
	ClusterReference *Reference `json:"cluster_reference,omitempty"`

	// Any error messages for the vm, if in an error state.
	MessageList []*MessageResource `json:"message_list,omitempty"`

	// Name of the recovery point.
	Name string `json:"name,omitempty"`

	// resources
	Resources *VMRecoveryPointDefStatusResources `json:"resources,omitempty"`

	// The state of the vm recovery point.
	State string `json:"state,omitempty"`
}

type VMRecoveryPointResources struct {

	// The time when the the recovery point is created. This is in internet
	// date/time format (RFC 3339). For example, 1985-04-12T23:20:50.52Z,
	// this represents 20 minutes and 50.52 seconds after the 23rd hour of
	// April 12th, 1985 in UTC.
	//
	// Format: date-time
	CreationTime *time.Time `json:"creation_time,omitempty"`

	// The time when this recovery point expires and will be garbage
	// collected. This is in internet date/time format (RFC 3339). For
	// example, 1985-04-12T23:20:50.52Z, this represents 20 minutes and
	// 50.52 seconds after the 23rd hour of April 12th, 1985 in UTC. If not
	// set, then the recovery point never expires.
	//
	// Format: date-time
	ExpirationTime *time.Time `json:"expiration_time,omitempty"`

	// Reference to vm that this recovery point is a snapshot of.
	//
	ParentVMReference *Reference `json:"parent_vm_reference,omitempty"`

	// Crash consistent or Application Consistent recovery point
	RecoveryPointType string `json:"recovery_point_type,omitempty"`

	// Reference to the availability zone where the source recovery
	// point exists. This need to be set to copy a recovery from some
	// other location.
	//
	SourceAvailabilityZoneReference *Reference `json:"source_availability_zone_reference,omitempty"`

	// Reference to the cluster in the source availability zone.
	//
	SourceClusterReference *Reference `json:"source_cluster_reference,omitempty"`

	// Location agnostic UUID of the recovery point. If a recovery
	// point is replicated to a different clusters, then all the
	// instances of same recovery point will share this UUID.
	//
	// Pattern: ^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$
	VMRecoveryPointLocationAgnosticUUID string `json:"vm_recovery_point_location_agnostic_uuid,omitempty"`
}

type VMRecoveryPointDefStatusResources struct {

	// This field is same for all the entities (irrespective of kind) that
	// were snapshotted together.
	//
	// Pattern: ^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$
	ConsistencyGroupUUID string `json:"consistency_group_uuid,omitempty"`

	// The time when the the recovery point is created. This is in internet
	// date/time format (RFC 3339). For example, 1985-04-12T23:20:50.52Z,
	// this represents 20 minutes and 50.52 seconds after the 23rd hour of
	// April 12th, 1985 in UTC.
	//
	CreationTime *time.Time `json:"creation_time,omitempty"`

	// The time when this recovery point expires and will be garbage
	// collected. This is in internet date/time format (RFC 3339). For
	// example, 1985-04-12T23:20:50.52Z, this represents 20 minutes and
	// 50.52 seconds after the 23rd hour of April 12th, 1985 in UTC. If not
	// set, then the recovery point never expires.
	//
	// Format: date-time
	ExpirationTime *time.Time `json:"expiration_time,omitempty"`

	// Reference to vm that this recovery point is a snapshot of.
	//
	ParentVMReference *Reference `json:"parent_vm_reference,omitempty"`

	// Crash consistent or Application Consistent recovery point
	RecoveryPointType string `json:"recovery_point_type,omitempty"`

	// Reference to the availability zone where the source recovery
	// point exists. This need to be set to copy a recovery from some
	// other location.
	//
	SourceAvailabilityZoneReference *Reference `json:"source_availability_zone_reference,omitempty"`

	// Reference to the cluster in the source availability zone. This
	// need to be set to copy a recovery from some other location.
	//
	SourceClusterReference *Reference `json:"source_cluster_reference,omitempty"`

	// Metadata of the vm at the time of snapshot.
	//
	VMMetadata *Metadata `json:"vm_metadata,omitempty"`

	// Location agnostic UUID of the recovery point. If a recovery
	// point is replicated to a different clusters, then all the
	// instances of same recovery point will share this UUID.
	//
	// Pattern: ^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$
	VMRecoveryPointLocationAgnosticUUID *string `json:"vm_recovery_point_location_agnostic_uuid,omitempty"`

	// Spec of the vm at the time of snapshot.
	//
	VMSpec *VM `json:"vm_spec,omitempty"`
}
