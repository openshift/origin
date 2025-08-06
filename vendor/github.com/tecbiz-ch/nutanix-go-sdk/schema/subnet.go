package schema

// SubnetResources represents Subnet creation/modification spec.
type SubnetResources struct {
	IPConfig *IPConfig `json:"ip_config,omitempty"`

	NetworkFunctionChainReference *Reference `json:"network_function_chain_reference,omitempty"`

	SubnetType string `json:"subnet_type,omitempty"`

	VPCReference *Reference `json:"vpc_reference,omitempty"`

	VlanID *int64 `json:"vlan_id,omitempty"`

	VswitchName string `json:"vswitch_name,omitempty"`
}

// Subnet An intentful representation of a subnet spec
type Subnet struct {
	AvailabilityZoneReference *Reference `json:"availability_zone_reference,omitempty"`

	ClusterReference *Reference `json:"cluster_reference,omitempty"`

	// A description for subnet.
	Description string `json:"description,omitempty"`

	// subnet Name.
	Name string `json:"name"`

	Resources *SubnetResources `json:"resources,omitempty"`
}

// SubnetStatus represents The status of a REST API call. Only used when there is a failure to report.
type SubnetStatus struct {
	APIVersion string `json:"api_version,omitempty"`

	// The HTTP error code.
	Code int64 `json:"code,omitempty"`

	// The kind name
	Kind string `json:"kind,omitempty"`

	MessageList []*MessageResource `json:"message_list,omitempty"`

	State string `json:"state,omitempty"`
}

// SubnetResourcesDefStatus represents a Subnet creation/modification status.
type SubnetResourcesDefStatus struct {
	IPConfig *IPConfig `json:"ip_config,omitempty"`

	NetworkFunctionChainReference *Reference `json:"network_function_chain_reference,omitempty"`

	SubnetType string `json:"subnet_type"`

	VlanID int64 `json:"vlan_id,omitempty"`

	VswitchName string `json:"vswitch_name,omitempty"`
}

// SubnetDefStatus An intentful representation of a subnet status
type SubnetDefStatus struct {
	AvailabilityZoneReference *Reference `json:"availability_zone_reference,omitempty"`

	ClusterReference *Reference `json:"cluster_reference,omitempty"`

	// A description for subnet.
	Description string `json:"description"`

	// Any error messages for the subnet, if in an error state.
	MessageList []*MessageResource `json:"message_list,omitempty"`

	// subnet Name.
	Name string `json:"name"`

	Resources *SubnetResourcesDefStatus `json:"resources,omitempty"`

	// The state of the subnet.
	State string `json:"state,omitempty"`

	ExecutionContext *ExecutionContext `json:"execution_context,omitempty"`
}

// IPPool represents IP pool.
type IPPool struct {
	// Range of IPs (example: 10.0.0.9 10.0.0.19).
	Range string `json:"range,omitempty"`
}

// DHCPOptions Spec for defining DHCP options.
type DHCPOptions struct {
	BootFileName string `json:"boot_file_name,omitempty"`

	DomainName string `json:"domain_name,omitempty"`

	DomainNameServerList []string `json:"domain_name_server_list,omitempty"`

	DomainSearchList []string `json:"domain_search_list,omitempty"`

	TFTPServerName string `json:"tftp_server_name,omitempty"`
}

// IPConfig represents the configurtion of IP.
type IPConfig struct {

	// Default gateway IP address.
	DefaultGatewayIP string `json:"default_gateway_ip,omitempty"`

	DHCPOptions *DHCPOptions `json:"dhcp_options,omitempty"`

	DHCPServerAddress *Address `json:"dhcp_server_address,omitempty"`

	PoolList []*IPPool `json:"pool_list,omitempty"`

	PrefixLength int64 `json:"prefix_length,omitempty"`

	// Subnet IP address.
	SubnetIP string `json:"subnet_ip,omitempty"`
}

// SubnetIntent represents the response object for intentful operations on a subnet
type SubnetIntent struct {
	APIVersion string `json:"api_version,omitempty"`

	Metadata *Metadata `json:"metadata,omitempty"`

	Spec *Subnet `json:"spec,omitempty"`

	Status *SubnetDefStatus `json:"status,omitempty"`
}

type SubnetIntentRequest struct {
	APIVersion string `json:"api_version,omitempty"`

	Metadata *Metadata `json:"metadata,omitempty"`

	Spec *Subnet `json:"spec,omitempty"`
}

// SubnetListIntent represents the response object for intentful operation of subnets
type SubnetListIntent struct {
	APIVersion string `json:"api_version"`

	Entities []*SubnetIntent `json:"entities,omitempty"`

	Metadata *ListMetadata `json:"metadata"`
}
