package schema

type FloatingIP struct {

	// availability zone reference
	AvailabilityZoneReference *Reference `json:"availability_zone_reference,omitempty"`

	// A description for floating_ip.
	// Max Length: 1000
	Description string `json:"description,omitempty"`

	// floating_ip Name.
	// Max Length: 256
	Name string `json:"name,omitempty"`

	// resources
	// Required: true
	Resources *FloatingIPResources `json:"resources"`
}

// FloatingIPResourcesDefStatus Floating IP allocation status
//
// Floating IP allocation status.
//
// swagger:model floating_ip_resources_def_status
type FloatingIPResources struct {

	// External subnet from which floating IP is selected.
	ExternalSubnetReference *Reference `json:"external_subnet_reference,omitempty"`

	// The Floating IP associated with the vnic.
	// Pattern: ^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$
	FloatingIP string `json:"floating_ip,omitempty"`

	// Private IP with which the floating IP is associated.
	// Pattern: ^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$
	PrivateIP string `json:"private_ip,omitempty"`

	// NIC with which the floating IP is associated.
	VMNicReference *Reference `json:"vm_nic_reference,omitempty"`

	// VPC in which the private IP exists.
	VpcReference *Reference `json:"vpc_reference,omitempty"`
}

type FloatingIPIntent struct {

	// api version
	APIVersion string `json:"api_version,omitempty"`

	// metadata
	// Required: true
	Metadata *Metadata `json:"metadata"`

	// spec
	Spec *FloatingIP `json:"spec,omitempty"`

	// status
	Status *FloatingIPDefStatus `json:"status,omitempty"`
}

// FloatingIPDefStatus floating_ip Intent Status with placement specified
//
// An intentful representation of a floating_ip status
//
// swagger:model floating_ip_def_status
type FloatingIPDefStatus struct {

	// availability zone reference
	AvailabilityZoneReference *Reference `json:"availability_zone_reference,omitempty"`

	// cluster reference
	ClusterReference *Reference `json:"cluster_reference,omitempty"`

	// A description for floating_ip.
	Description string `json:"description,omitempty"`

	// Any error messages for the floating_ip, if in an error state.
	MessageList []*MessageResource `json:"message_list"`

	// floating_ip Name.
	// Required: true
	Name *string `json:"name"`

	// resources
	// Required: true
	Resources *FloatingIPResources `json:"resources"`

	// The state of the floating_ip.
	State string `json:"state,omitempty"`
}

// FloatingIPListIntentResponse Entity Intent List Response
//
// Response object for intentful operation of floating_ips
//
// swagger:model floating_ip_list_intent_response
type FloatingIPListIntent struct {

	// api version
	// Required: true
	APIVersion string `json:"api_version,omitempty"`

	// entities
	Entities []*FloatingIPIntent `json:"entities"`

	// metadata
	// Required: true
	Metadata *ListMetadata `json:"metadata"`
}
