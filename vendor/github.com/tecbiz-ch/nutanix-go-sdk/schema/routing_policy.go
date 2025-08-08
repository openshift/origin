package schema

type RoutingPolicy struct {

	// availability zone reference
	AvailabilityZoneReference *Reference `json:"availability_zone_reference,omitempty"`

	// A description for routing_policy.
	// Max Length: 1000
	Description string `json:"description,omitempty"`

	// routing_policy Name.
	// Required: true
	// Max Length: 256
	Name string `json:"name"`

	// resources
	// Required: true
	Resources *RoutingPolicyResources `json:"resources"`
}

// Routing Policy creation/modification spec. The routing policy matches incoming traffic on the router based on the following fields: 'source', 'destination', 'protocol_type', 'protocol_parameters'. Once traffic matches the parameters defined in the policy 'action' field defines the action that needs to be performed on the traffic. 'action' could be permit/deny/reroute.
//
// swagger:model routing_policy_resources
type RoutingPolicyResources struct {

	// The IP protocol type of traffic that is entering the router.
	//
	// Required: true
	Action *RoutingPolicyAction `json:"action"`

	// Destination of traffic that is entering the router.
	//
	// Required: true
	Destination *NetworkAddress `json:"destination"`

	// Whether to configure/install policy in reverse direction too (i.e matching traffic from destination to source)
	//
	IsBidirectional bool `json:"is_bidirectional,omitempty"`

	// priority
	// Required: true
	// Maximum: 1000
	// Minimum: 1
	Priority int16 `json:"priority"`

	// The IP protocol type of traffic that is entering the router.
	//
	ProtocolParameters *ProtocolParameters `json:"protocol_parameters,omitempty"`

	// protocol type
	// Required: true
	ProtocolType string `json:"protocol_type"`

	// Source of traffic that is entering the router.
	//
	// Required: true
	Source *NetworkAddress `json:"source"`

	// The virtual network this routing policy belongs to. This reference is deprecated, use vpc_reference instead.
	//
	VirtualNetworkReference *Reference `json:"virtual_network_reference,omitempty"`

	// The VPC this routing policy belongs to.
	//
	VpcReference *Reference `json:"vpc_reference,omitempty"`
}

// NetworkAddress Network address
//
// Address (source/destination) of an IP packet. > This could be either an ip prefix or a special category like "INTERNET".
//
// swagger:model network_address
type NetworkAddress struct {

	// address type
	AddressType string `json:"address_type,omitempty"`

	// ip subnet
	IPSubnet *IPSubnet `json:"ip_subnet,omitempty"`
}

// RoutingPolicyAction Action
//
// # Routing policy action
//
// swagger:model routing_policy_action
type RoutingPolicyAction struct {

	// action
	// Required: true
	Action string `json:"action"`

	// IP addresses of network services in the chain.> This field is valid only when action is REROUTE.
	ServiceIPList []string `json:"service_ip_list,omitempty"`
}

// ProtocolParameters IP protocol
//
// # Routing policy IP protocol parameters
//
// swagger:model protocol_parameters
type ProtocolParameters struct {

	// ICMP parameters to be matched
	Icmp *Icmp `json:"icmp,omitempty"`

	// protocol number
	// Maximum: 255
	// Minimum: 0
	ProtocolNumber *uint8 `json:"protocol_number,omitempty"`

	// TCP parameters to be matched
	TCP *TCP `json:"tcp,omitempty"`

	// UDP parameters to be matched
	UDP *UDP `json:"udp,omitempty"`
}

// Icmp ICMP parameters
//
// ICMP parameters to be matched in routing policy.
//
// swagger:model icmp
type Icmp struct {

	// icmp code
	// Maximum: 255
	// Minimum: 0
	IcmpCode *uint8 `json:"icmp_code,omitempty"`

	// icmp type
	// Maximum: 255
	// Minimum: 0
	IcmpType *uint8 `json:"icmp_type,omitempty"`
}

// UDP UDP parameters
//
// # UDP parameters to be matched in routing policy
//
// swagger:model udp
type UDP struct {

	// Range of UDP destination ports. This field is deprecated, use destination_port_range_list instead.
	//
	DestinationPortRange *PortRange `json:"destination_port_range,omitempty"`

	// List of ranges of UDP destination ports.
	DestinationPortRangeList []*PortRange `json:"destination_port_range_list"`

	// Range of UDP source ports. This field is deprecated, use source_port_range_list instead.
	//
	SourcePortRange *PortRange `json:"source_port_range,omitempty"`

	// List of ranges of UDP source ports.
	SourcePortRangeList []*PortRange `json:"source_port_range_list"`
}

// TCP TCP parameters
//
// TCP parameters to be matched in routing policy.
//
// swagger:model tcp
type TCP struct {

	// Range of TCP destination ports. This field is deprecated, use destination_port_range_list instead.
	//
	DestinationPortRange *PortRange `json:"destination_port_range,omitempty"`

	// List of ranges of TCP destination ports.
	DestinationPortRangeList []*PortRange `json:"destination_port_range_list"`

	// Range of TCP source ports. This field is deprecated, use source_port_range_list instead.
	//
	SourcePortRange *PortRange `json:"source_port_range,omitempty"`

	// List of ranges of TCP source ports.
	SourcePortRangeList []*PortRange `json:"source_port_range_list"`
}

type RoutingPolicyIntent struct {

	// api version
	// Required: true
	APIVersion string `json:"api_version"`

	Metadata *Metadata `json:"metadata,omitempty"`

	// spec
	Spec *RoutingPolicy `json:"spec,omitempty"`

	// status
	Status *RoutingPolicyDefStatus `json:"status,omitempty"`
}

type RoutingPolicyDefStatus struct {

	// availability zone reference
	AvailabilityZoneReference *Reference `json:"availability_zone_reference,omitempty"`

	// cluster reference
	ClusterReference *Reference `json:"cluster_reference,omitempty"`

	// A description for routing_policy.
	Description string `json:"description,omitempty"`

	// Any error messages for the routing_policy, if in an error state.
	MessageList []*MessageResource `json:"message_list"`

	// routing_policy Name.
	// Required: true
	Name *string `json:"name"`

	// resources
	// Required: true
	Resources *RoutingPolicyResourcesDefStatus `json:"resources"`

	// The state of the routing_policy.
	State string `json:"state,omitempty"`
}

type RoutingPolicyResourcesDefStatus struct {

	// The action to be taken on traffic entering the router.
	//
	Action *RoutingPolicyAction `json:"action,omitempty"`

	// The destination IP address of traffic that is entering the router.
	//
	Destination *NetworkAddress `json:"destination,omitempty"`

	// Error message describing why the routing policy is inactive.
	//
	ErrorMessage string `json:"error_message,omitempty"`

	// Whether to configure/install policy in reverse direction too (i.e matching traffic from destination to source)
	//
	IsBidirectional bool `json:"is_bidirectional,omitempty"`

	// priority
	Priority int16 `json:"priority,omitempty"`

	// IP protocol parameters of traffic entering the router.
	//
	ProtocolParameters *ProtocolParameters `json:"protocol_parameters,omitempty"`

	// The IP protocol type of traffic that is entering the router.
	//
	ProtocolType string `json:"protocol_type,omitempty"`

	// Number of packets matching the routing policy.
	RoutingPolicyCounters *RoutingPolicyCounters `json:"routing_policy_counters,omitempty"`

	// Number of packets matching the reverse direction routing policy. Applicable only if is_bidirectional is true.
	//
	RoutingPolicyCountersReverseDirection *RoutingPolicyCounters `json:"routing_policy_counters_reverse_direction,omitempty"`

	// Policy counters for each service IP.
	ServiceIPCounters []*ServiceIPCounters `json:"service_ip_counters"`

	// Policy counters for each service IP for reverse direction reroute routing policy. Applicable only if is_bidirectional is true.
	//
	ServiceIPCountersReverseDirection []*ServiceIPCounters `json:"service_ip_counters_reverse_direction"`

	// The source IP address of traffic that is entering the router.
	//
	Source *NetworkAddress `json:"source,omitempty"`
	// The virtual network this routing policy belongs to. This reference is deprecated, use vpc_reference instead.
	//
	VirtualNetworkReference *Reference `json:"virtual_network_reference,omitempty"`

	// The VPC this routing policy belongs to.
	//
	VpcReference *Reference `json:"vpc_reference,omitempty"`
}

// RoutingPolicyCounters Policy counters
//
// Number of packets and bytes hitting the rule.
//
// swagger:model routing_policy_counters
type RoutingPolicyCounters struct {

	// byte count
	ByteCount uint64 `json:"byte_count,omitempty"`

	// packet count
	PacketCount uint64 `json:"packet_count,omitempty"`
}

// ServiceIPCounters Service IP counters
//
// Number of packets and bytes hitting the service IP.
//
// swagger:model service_ip_counters
type ServiceIPCounters struct {

	// received
	Received *RoutingPolicyCounters `json:"received,omitempty"`

	// sent
	Sent *RoutingPolicyCounters `json:"sent,omitempty"`

	// service ip
	// Pattern: ^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$
	ServiceIP string `json:"service_ip,omitempty"`
}

type RoutingPolicyListIntent struct {

	// api version
	// Required: true
	APIVersion string `json:"api_version"`

	// entities
	Entities []*RoutingPolicyIntent `json:"entities"`

	// metadata
	// Required: true
	Metadata *ListMetadata `json:"metadata"`
}
