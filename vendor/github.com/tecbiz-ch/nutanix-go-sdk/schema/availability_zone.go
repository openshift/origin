package schema

type AvailabilityZoneListIntent struct {

	// api version
	// Required: true
	APIVersion *string `json:"api_version,omitempty"`

	// entities
	Entities []*AvailabilityZoneIntent `json:"entities,omitempty"`

	// metadata
	// Required: true
	Metadata *ListMetadata `json:"metadata"`
}

type AvailabilityZoneIntent struct {

	// api version
	APIVersion *string `json:"api_version,omitempty"`

	// metadata
	// Required: true
	Metadata *Metadata `json:"metadata"`

	// spec
	Spec *AvailabilityZone `json:"spec,omitempty"`

	// status
	Status *AvailabilityZoneDefStatus `json:"status,omitempty"`
}

type AvailabilityZoneDefStatus struct {

	// Availability Zone Name
	// Required: true
	Name *string `json:"name,omitempty"`

	// resources
	// Required: true
	Resources *AvailabilityZoneResources `json:"resources,omitempty"`
}

type AvailabilityZoneResources struct {

	// credentials
	Credentials *AvailabilityZoneCredentials `json:"credentials,omitempty"`

	// Display name. It is mainly used by user interface to show the
	// user-friendly name of the availability zone. If unset, default value
	// will be used.
	//
	DisplayName *string `json:"display_name,omitempty"`

	// This defines the type of management entity. Its value can be Xi,
	// PC, or Local. Local AZs are auto-created and cannot be deleted.
	// How to talk to management entity will be decided based on the type
	// of management plane.
	//
	// Required: true
	ManagementPlaneType *string `json:"management_plane_type"`

	// Identifier of the management plane. This could be the URL of the
	// PC or the FQDN of Xi portal.
	//
	ManagementURL *string `json:"management_url,omitempty"`

	// Cloud region where the data will be replicated to. Based on the
	// cloud provider type the available list of regions will differ.
	//
	Region *string `json:"region,omitempty"`
}

type AvailabilityZone struct {

	// Availability Zone Name
	// Required: true
	Name *string `json:"name,omitempty"`

	// resources
	// Required: true
	Resources *AvailabilityZoneResources `json:"resources,omitempty"`
}

type AvailabilityZoneCredentials struct {
	Pc *AvailabilityZoneCredentialsPc `json:"pc,omitempty"`
}

type AvailabilityZoneCredentialsPc struct {
	RemoteConnectionReference *Reference `json:"remote_connection_reference,omitempty"`
}
