package schema

type VMSnapshotListIntent struct {

	// api version
	// Required: true
	APIVersion string `json:"api_version"`

	// entities
	Entities []*VMSnapshotIntent `json:"entities"`

	// metadata
	// Required: true
	Metadata *ListMetadata `json:"metadata"`
}

type VMSnapshotIntent struct {

	// api version
	APIVersion string `json:"api_version,omitempty"`

	// metadata
	// Required: true
	Metadata *Metadata `json:"metadata,omitempty"`

	// spec
	Spec *VMSnapshot `json:"spec,omitempty"`

	// status
	Status *VMSnapshotDefStatus `json:"status,omitempty"`
}

type VMSnapshot struct {

	// The time when this snapshot expires and will be garbage collected.
	// If not set, then the snapshot never expires.
	//
	ExpirationTimeMsecs int64 `json:"expiration_time_msecs,omitempty"`

	// Name of the snapshot
	// Max Length: 64
	Name string `json:"name,omitempty"`

	// resources
	Resources *VMSnapshotResources `json:"resources,omitempty"`

	// Crash consistent or Application Consistent snapshot
	SnapshotType string `json:"snapshot_type,omitempty"`
}

type VMSnapshotResources struct {

	// UUID of the base entity for which snapshot need to be taken
	//
	// Pattern: ^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$
	EntityUUID string `json:"entity_uuid,omitempty"`
}

type VMSnapshotDefStatus struct {

	// This field is same for all the entities (irrespective of kind) that
	// were snapshotted together.
	//
	// Pattern: ^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$
	ConsistencyGroupUUID string `json:"consistency_group_uuid,omitempty"`

	// The time when this snapshot expires and will be garbage collected.
	// If not set, then the snapshot never expires.
	//
	ExpirationTimeMsecs int64 `json:"expiration_time_msecs,omitempty"`

	// If a snapshot is replicated to a different clusters, then all the
	// instances of same snapshot will share this UUID.
	//
	// Pattern: ^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$
	LocationAgnosticUUID string `json:"location_agnostic_uuid,omitempty"`

	// Any error messages for the {kind}}, if in an error state.
	MessageList []*MessageResource `json:"message_list"`

	// Name of the snapshot
	Name string `json:"name,omitempty"`

	// resources
	Resources *VMSnapshotResources `json:"resources,omitempty"`

	// Describes the files that are included in the snapshot.
	//
	// Required: true
	SnapshotFileList []*VMSnapshotDefStatusSnapshotFileListItems `json:"snapshot_file_list"`

	// Crash consistent or Application Consistent snapshot
	SnapshotType string `json:"snapshot_type,omitempty"`

	// The state of the VM snapshot.
	State string `json:"state,omitempty"`
}

type VMSnapshotDefStatusSnapshotFileListItems struct {

	// Pathname of the file that participated in the snapshot.
	//
	FilePath string `json:"file_path,omitempty"`

	// Pathname of the snapshot of the file.
	SnapshotFilePath string `json:"snapshot_file_path,omitempty"`
}
