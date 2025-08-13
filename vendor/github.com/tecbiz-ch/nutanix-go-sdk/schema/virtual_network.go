package schema

type VirtualNetworkListIntent struct {

	// api version
	// Required: true
	APIVersion string `json:"api_version,omitempty"`

	// entities
	Entities []*VirtualNetworkIntent `json:"entities,omitempty"`

	// metadata
	// Required: true
	Metadata *ListMetadata `json:"metadata,omitempty"`
}

type VirtualNetworkIntent struct {

	// api version
	APIVersion string `json:"api_version,omitempty"`

	// metadata
	// Required: true
	Metadata *Metadata `json:"metadata,omitempty"`

	// spec
	Spec *VirtualNetwork `json:"spec,omitempty"`

	// status
	Status *VirtualNetworkDefStatus `json:"status,omitempty"`
}

type VirtualNetwork struct {

	// description
	// Max Length: 1000
	Description string `json:"description,omitempty"`

	// name
	// Required: true
	// Max Length: 64
	Name string `json:"name,omitempty"`

	// resources
	Resources *VirtualNetworkResources `json:"resources,omitempty"`
}

type VirtualNetworkResources struct {

	// List of availability zones in Xi from which resources are derived (Only supported on Xi)
	//
	AvailabilityZoneReferenceList []*Reference `json:"availability_zone_reference_list,omitempty"`

	// List of domain name server IPs.
	CommonDomainNameServerIPList []*Address `json:"common_domain_name_server_ip_list,omitempty"`

	// Per region providing secure connection from on-prem to Xi (Only supported on Xi)
	//
	VpnConfig string `json:"vpn_config,omitempty"`
}

type VirtualNetworkDefStatus struct {

	// description
	Description string `json:"description,omitempty"`

	// Any error messages for the virtual network, if in an error state.
	//
	MessageList []*MessageResource `json:"message_list,omitempty"`

	// name
	Name string `json:"name,omitempty"`

	// resources
	Resources *VirtualNetworkResourcesDefStatus `json:"resources,omitempty"`

	// The state of the virtual network.
	State string `json:"state,omitempty"`
}

type VirtualNetworkResourcesDefStatus struct {

	// List of availability zones in Xi from which resources are derived (Only supported on Xi)
	//
	AvailabilityZoneReferenceList []*Reference `json:"availability_zone_reference_list"`

	// List of domain name server IPs.
	CommonDomainNameServerIPList []*Address `json:"common_domain_name_server_ip_list"`

	// List of IP addresses used for SNAT.
	NatIPList []string `json:"nat_ip_list"`

	// Per region providing secure connection from on-prem to Xi (Only supported on Xi)
	//
	VpnConfig string `json:"vpn_config,omitempty"`
}
