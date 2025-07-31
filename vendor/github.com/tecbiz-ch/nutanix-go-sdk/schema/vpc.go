package schema

type Vpc struct {

	// description
	// Max Length: 1000
	Description string `json:"description,omitempty"`

	// name
	// Required: true
	// Max Length: 64
	Name string `json:"name"`

	// resources
	Resources *VpcResources `json:"resources,omitempty"`
}

type VpcResources struct {

	// List of availability zones in Xi from which resources are derived (Only supported on Xi)
	//
	AvailabilityZoneReferenceList []*Reference `json:"availability_zone_reference_list,omitempty"`

	// List of domain name server IPs.
	CommonDomainNameServerIPList []*Address `json:"common_domain_name_server_ip_list,omitempty"`

	// List of external subnets attached to this VPC.
	ExternalSubnetList []*ExternalSubnet `json:"external_subnet_list,omitempty"`

	// CIDR blocks from the VPC which can talk externally without performing NAT. These blocks should be between /16 netmask and /28 netmask and cannot overlap across VPCs. They are effective when the VPC connects to a NAT-less external subnet.
	//
	ExternallyRoutablePrefixList []*IPSubnet `json:"externally_routable_prefix_list,omitempty"`
}

type Node struct {

	// host reference
	// Required: true
	HostReference *Reference `json:"host_reference"`

	// Node IP Address
	// Pattern: ^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$
	IPAddress string `json:"ip_address,omitempty"`
}

// ExternalSubnetDefStatus External subnet status.
//
// External subnet status.
//
// swagger:model external_subnet_def_status
type ExternalSubnet struct {

	// Active gateway node for the external connectivity.
	ActiveGatewayNode *Node `json:"active_gateway_node,omitempty"`

	// List of IP addresses for the VPC router port connecting to the external network.
	//
	ExternalIPList []string `json:"external_ip_list,omitempty"`

	// External subnet reference.
	ExternalSubnetReference *Reference `json:"external_subnet_reference,omitempty"`

	// List of gateway nodes for the external connectivity.
	GatewayNodeUUIDList []string `json:"gateway_node_uuid_list,omitempty"`
}

// swagger:model vpc_list_intent_response
type VpcListIntent struct {

	// api version
	// Required: true
	APIVersion string `json:"api_version,omitempty"`

	// entities
	Entities []*VpcIntent `json:"entities"`

	// metadata
	// Required: true
	Metadata *ListMetadata `json:"metadata"`
}

// swagger:model vpc_intent_resource
type VpcIntent struct {

	// api version
	APIVersion string `json:"api_version,omitempty"`

	// metadata
	// Required: true
	Metadata *Metadata `json:"metadata,omitempty"`

	// spec
	Spec *Vpc `json:"spec,omitempty"`

	// status
	Status *VpcDefStatus `json:"status,omitempty"`
}

type VpcDefStatus struct {

	// description
	Description string `json:"description,omitempty"`

	// Any error messages for the VPC, if in an error state.
	//
	MessageList []*MessageResource `json:"message_list"`

	// name
	Name string `json:"name,omitempty"`

	// resources
	Resources *VpcResourcesDefStatus `json:"resources,omitempty"`

	// The state of the VPC.
	State string `json:"state,omitempty"`
}

// VpcResourcesDefStatus VPC resources status
//
// # VPC resources status
//
// swagger:model vpc_resources_def_status
type VpcResourcesDefStatus struct {

	// List of availability zones in Xi from which resources are derived (Only supported on Xi)
	//
	AvailabilityZoneReferenceList []*Reference `json:"availability_zone_reference_list"`

	// List of domain name server IPs.
	CommonDomainNameServerIPList []*Address `json:"common_domain_name_server_ip_list"`

	// List of external subnets attached to this VPC.
	ExternalSubnetList []*ExternalSubnet `json:"external_subnet_list"`

	// CIDR blocks from the VPC which can talk externally without performing NAT. These blocks should be between /16 netmask and /28 netmask and cannot overlap across VPCs. They are effective when the VPC connects to a NAT-less external subnet.
	//
	ExternallyRoutablePrefixList []*IPSubnet `json:"externally_routable_prefix_list"`

	// List of IP addresses used for SNAT.
	NatIPList []string `json:"nat_ip_list"`
}
