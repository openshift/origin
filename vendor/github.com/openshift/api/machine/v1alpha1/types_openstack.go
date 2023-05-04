/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenstackProviderSpec is the type that will be embedded in a Machine.Spec.ProviderSpec field
// for an OpenStack Instance. It is used by the Openstack machine actuator to create a single machine instance.
// +k8s:openapi-gen=true
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type OpenstackProviderSpec struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The name of the secret containing the openstack credentials
	CloudsSecret *corev1.SecretReference `json:"cloudsSecret"`

	// The name of the cloud to use from the clouds secret
	CloudName string `json:"cloudName"`

	// The flavor reference for the flavor for your server instance.
	Flavor string `json:"flavor"`

	// The name of the image to use for your server instance.
	// If the RootVolume is specified, this will be ignored and use rootVolume directly.
	Image string `json:"image"`

	// The ssh key to inject in the instance
	KeyName string `json:"keyName,omitempty"`

	// The machine ssh username
	SshUserName string `json:"sshUserName,omitempty"`

	// A networks object. Required parameter when there are multiple networks defined for the tenant.
	// When you do not specify the networks parameter, the server attaches to the only network created for the current tenant.
	Networks []NetworkParam `json:"networks,omitempty"`

	// Create and assign additional ports to instances
	Ports []PortOpts `json:"ports,omitempty"`

	// floatingIP specifies a floating IP to be associated with the machine.
	// Note that it is not safe to use this parameter in a MachineSet, as
	// only one Machine may be assigned the same floating IP.
	//
	// Deprecated: floatingIP will be removed in a future release as it cannot be implemented correctly.
	FloatingIP string `json:"floatingIP,omitempty"`

	// The availability zone from which to launch the server.
	AvailabilityZone string `json:"availabilityZone,omitempty"`

	// The names of the security groups to assign to the instance
	SecurityGroups []SecurityGroupParam `json:"securityGroups,omitempty"`

	// The name of the secret containing the user data (startup script in most cases)
	UserDataSecret *corev1.SecretReference `json:"userDataSecret,omitempty"`

	// Whether the server instance is created on a trunk port or not.
	Trunk bool `json:"trunk,omitempty"`

	// Machine tags
	// Requires Nova api 2.52 minimum!
	Tags []string `json:"tags,omitempty"`

	// Metadata mapping. Allows you to create a map of key value pairs to add to the server instance.
	ServerMetadata map[string]string `json:"serverMetadata,omitempty"`

	// Config Drive support
	ConfigDrive *bool `json:"configDrive,omitempty"`

	// The volume metadata to boot from
	RootVolume *RootVolume `json:"rootVolume,omitempty"`

	// The server group to assign the machine to.
	ServerGroupID string `json:"serverGroupID,omitempty"`

	// The server group to assign the machine to. A server group with that
	// name will be created if it does not exist. If both ServerGroupID and
	// ServerGroupName are non-empty, they must refer to the same OpenStack
	// resource.
	ServerGroupName string `json:"serverGroupName,omitempty"`

	// The subnet that a set of machines will get ingress/egress traffic from
	PrimarySubnet string `json:"primarySubnet,omitempty"`
}

type SecurityGroupParam struct {
	// Security Group UUID
	UUID string `json:"uuid,omitempty"`
	// Security Group name
	Name string `json:"name,omitempty"`
	// Filters used to query security groups in openstack
	Filter SecurityGroupFilter `json:"filter,omitempty"`
}

type SecurityGroupFilter struct {
	// id specifies the ID of a security group to use. If set, id will not
	// be validated before use. An invalid id will result in failure to
	// create a server with an appropriate error message.
	ID string `json:"id,omitempty"`
	// name filters security groups by name.
	Name string `json:"name,omitempty"`
	// description filters security groups by description.
	Description string `json:"description,omitempty"`
	// tenantId filters security groups by tenant ID.
	// Deprecated: use projectId instead. tenantId will be ignored if projectId is set.
	TenantID string `json:"tenantId,omitempty"`
	// projectId filters security groups by project ID.
	ProjectID string `json:"projectId,omitempty"`
	// tags filters by security groups containing all specified tags.
	// Multiple tags are comma separated.
	Tags string `json:"tags,omitempty"`
	// tagsAny filters by security groups containing any specified tags.
	// Multiple tags are comma separated.
	TagsAny string `json:"tagsAny,omitempty"`
	// notTags filters by security groups which don't match all specified tags. NOT (t1 AND t2...)
	// Multiple tags are comma separated.
	NotTags string `json:"notTags,omitempty"`
	// notTagsAny filters by security groups which don't match any specified tags. NOT (t1 OR t2...)
	// Multiple tags are comma separated.
	NotTagsAny string `json:"notTagsAny,omitempty"`

	// Deprecated: limit is silently ignored. It has no replacement.
	DeprecatedLimit int `json:"limit,omitempty"`
	// Deprecated: marker is silently ignored. It has no replacement.
	DeprecatedMarker string `json:"marker,omitempty"`
	// Deprecated: sortKey is silently ignored. It has no replacement.
	DeprecatedSortKey string `json:"sortKey,omitempty"`
	// Deprecated: sortDir is silently ignored. It has no replacement.
	DeprecatedSortDir string `json:"sortDir,omitempty"`
}

type NetworkParam struct {
	// The UUID of the network. Required if you omit the port attribute.
	UUID string `json:"uuid,omitempty"`
	// A fixed IPv4 address for the NIC.
	FixedIp string `json:"fixedIp,omitempty"`
	// Filters for optional network query
	Filter Filter `json:"filter,omitempty"`
	// Subnet within a network to use
	Subnets []SubnetParam `json:"subnets,omitempty"`
	// NoAllowedAddressPairs disables creation of allowed address pairs for the network ports
	NoAllowedAddressPairs bool `json:"noAllowedAddressPairs,omitempty"`
	// PortTags allows users to specify a list of tags to add to ports created in a given network
	PortTags []string `json:"portTags,omitempty"`
	// The virtual network interface card (vNIC) type that is bound to the
	// neutron port.
	VNICType string `json:"vnicType,omitempty"`
	// A dictionary that enables the application running on the specified
	// host to pass and receive virtual network interface (VIF) port-specific
	// information to the plug-in.
	Profile map[string]string `json:"profile,omitempty"`
	// PortSecurity optionally enables or disables security on ports managed by OpenStack
	PortSecurity *bool `json:"portSecurity,omitempty"`
}

type Filter struct {
	// Deprecated: use NetworkParam.uuid instead. Ignored if NetworkParam.uuid is set.
	ID string `json:"id,omitempty"`
	// name filters networks by name.
	Name string `json:"name,omitempty"`
	// description filters networks by description.
	Description string `json:"description,omitempty"`
	// tenantId filters networks by tenant ID.
	// Deprecated: use projectId instead. tenantId will be ignored if projectId is set.
	TenantID string `json:"tenantId,omitempty"`
	// projectId filters networks by project ID.
	ProjectID string `json:"projectId,omitempty"`
	// tags filters by networks containing all specified tags.
	// Multiple tags are comma separated.
	Tags string `json:"tags,omitempty"`
	// tagsAny filters by networks containing any specified tags.
	// Multiple tags are comma separated.
	TagsAny string `json:"tagsAny,omitempty"`
	// notTags filters by networks which don't match all specified tags. NOT (t1 AND t2...)
	// Multiple tags are comma separated.
	NotTags string `json:"notTags,omitempty"`
	// notTagsAny filters by networks which don't match any specified tags. NOT (t1 OR t2...)
	// Multiple tags are comma separated.
	NotTagsAny string `json:"notTagsAny,omitempty"`

	// Deprecated: status is silently ignored. It has no replacement.
	DeprecatedStatus string `json:"status,omitempty"`
	// Deprecated: adminStateUp is silently ignored. It has no replacement.
	DeprecatedAdminStateUp *bool `json:"adminStateUp,omitempty"`
	// Deprecated: shared is silently ignored. It has no replacement.
	DeprecatedShared *bool `json:"shared,omitempty"`
	// Deprecated: marker is silently ignored. It has no replacement.
	DeprecatedMarker string `json:"marker,omitempty"`
	// Deprecated: limit is silently ignored. It has no replacement.
	DeprecatedLimit int `json:"limit,omitempty"`
	// Deprecated: sortKey is silently ignored. It has no replacement.
	DeprecatedSortKey string `json:"sortKey,omitempty"`
	// Deprecated: sortDir is silently ignored. It has no replacement.
	DeprecatedSortDir string `json:"sortDir,omitempty"`
}

type SubnetParam struct {
	// The UUID of the network. Required if you omit the port attribute.
	UUID string `json:"uuid,omitempty"`

	// Filters for optional network query
	Filter SubnetFilter `json:"filter,omitempty"`

	// PortTags are tags that are added to ports created on this subnet
	PortTags []string `json:"portTags,omitempty"`

	// PortSecurity optionally enables or disables security on ports managed by OpenStack
	PortSecurity *bool `json:"portSecurity,omitempty"`
}

type SubnetFilter struct {
	// id is the uuid of a specific subnet to use. If specified, id will not
	// be validated. Instead server creation will fail with an appropriate
	// error.
	ID string `json:"id,omitempty"`
	// name filters subnets by name.
	Name string `json:"name,omitempty"`
	// description filters subnets by description.
	Description string `json:"description,omitempty"`
	// Deprecated: networkId is silently ignored. Set uuid on the containing network definition instead.
	NetworkID string `json:"networkId,omitempty"`
	// tenantId filters subnets by tenant ID.
	// Deprecated: use projectId instead. tenantId will be ignored if projectId is set.
	TenantID string `json:"tenantId,omitempty"`
	// projectId filters subnets by project ID.
	ProjectID string `json:"projectId,omitempty"`
	// ipVersion filters subnets by IP version.
	IPVersion int `json:"ipVersion,omitempty"`
	// gateway_ip filters subnets by gateway IP.
	GatewayIP string `json:"gateway_ip,omitempty"`
	// cidr filters subnets by CIDR.
	CIDR string `json:"cidr,omitempty"`
	// ipv6AddressMode filters subnets by IPv6 address mode.
	IPv6AddressMode string `json:"ipv6AddressMode,omitempty"`
	// ipv6RaMode filters subnets by IPv6 router adversiement mode.
	IPv6RAMode string `json:"ipv6RaMode,omitempty"`
	// subnetpoolId filters subnets by subnet pool ID.
	SubnetPoolID string `json:"subnetpoolId,omitempty"`
	// tags filters by subnets containing all specified tags.
	// Multiple tags are comma separated.
	Tags string `json:"tags,omitempty"`
	// tagsAny filters by subnets containing any specified tags.
	// Multiple tags are comma separated.
	TagsAny string `json:"tagsAny,omitempty"`
	// notTags filters by subnets which don't match all specified tags. NOT (t1 AND t2...)
	// Multiple tags are comma separated.
	NotTags string `json:"notTags,omitempty"`
	// notTagsAny filters by subnets which don't match any specified tags. NOT (t1 OR t2...)
	// Multiple tags are comma separated.
	NotTagsAny string `json:"notTagsAny,omitempty"`

	// Deprecated: enableDhcp is silently ignored. It has no replacement.
	DeprecatedEnableDHCP *bool `json:"enableDhcp,omitempty"`
	// Deprecated: limit is silently ignored. It has no replacement.
	DeprecatedLimit int `json:"limit,omitempty"`
	// Deprecated: marker is silently ignored. It has no replacement.
	DeprecatedMarker string `json:"marker,omitempty"`
	// Deprecated: sortKey is silently ignored. It has no replacement.
	DeprecatedSortKey string `json:"sortKey,omitempty"`
	// Deprecated: sortDir is silently ignored. It has no replacement.
	DeprecatedSortDir string `json:"sortDir,omitempty"`
}

type PortOpts struct {
	// networkID is the ID of the network the port will be created in. It is required.
	// +required
	NetworkID string `json:"networkID"`
	// If nameSuffix is specified the created port will be named <machine name>-<nameSuffix>.
	// If not specified the port will be named <machine-name>-<index of this port>.
	NameSuffix string `json:"nameSuffix,omitempty"`
	// description specifies the description of the created port.
	Description string `json:"description,omitempty"`
	// adminStateUp sets the administrative state of the created port to up (true), or down (false).
	AdminStateUp *bool `json:"adminStateUp,omitempty"`
	// macAddress specifies the MAC address of the created port.
	MACAddress string `json:"macAddress,omitempty"`
	// fixedIPs specifies a set of fixed IPs to assign to the port. They must all be valid for the port's network.
	FixedIPs []FixedIPs `json:"fixedIPs,omitempty"`
	// tenantID specifies the tenant ID of the created port. Note that this
	// requires OpenShift to have administrative permissions, which is
	// typically not the case. Use of this field is not recommended.
	// Deprecated: use projectID instead. It will be ignored if projectID is set.
	TenantID string `json:"tenantID,omitempty"`
	// projectID specifies the project ID of the created port. Note that this
	// requires OpenShift to have administrative permissions, which is
	// typically not the case. Use of this field is not recommended.
	ProjectID string `json:"projectID,omitempty"`
	// securityGroups specifies a set of security group UUIDs to use instead
	// of the machine's default security groups. The default security groups
	// will be used if this is left empty or not specified.
	SecurityGroups *[]string `json:"securityGroups,omitempty"`
	// allowedAddressPairs specifies a set of allowed address pairs to add to the port.
	AllowedAddressPairs []AddressPair `json:"allowedAddressPairs,omitempty"`
	// tags species a set of tags to add to the port.
	Tags []string `json:"tags,omitempty"`
	// The virtual network interface card (vNIC) type that is bound to the
	// neutron port.
	VNICType string `json:"vnicType,omitempty"`
	// A dictionary that enables the application running on the specified
	// host to pass and receive virtual network interface (VIF) port-specific
	// information to the plug-in.
	Profile map[string]string `json:"profile,omitempty"`
	// enable or disable security on a given port
	// incompatible with securityGroups and allowedAddressPairs
	PortSecurity *bool `json:"portSecurity,omitempty"`
	// Enables and disables trunk at port level. If not provided, openStackMachine.Spec.Trunk is inherited.
	Trunk *bool `json:"trunk,omitempty"`

	// The ID of the host where the port is allocated. Do not use this
	// field: it cannot be used correctly.
	// Deprecated: hostID is silently ignored. It will be removed with no replacement.
	DeprecatedHostID string `json:"hostID,omitempty"`
}

type AddressPair struct {
	IPAddress  string `json:"ipAddress,omitempty"`
	MACAddress string `json:"macAddress,omitempty"`
}

type FixedIPs struct {
	// subnetID specifies the ID of the subnet where the fixed IP will be allocated.
	SubnetID string `json:"subnetID"`
	// ipAddress is a specific IP address to use in the given subnet. Port
	// creation will fail if the address is not available. If not specified,
	// an available IP from the given subnet will be selected automatically.
	IPAddress string `json:"ipAddress,omitempty"`
}

type RootVolume struct {
	// sourceUUID specifies the UUID of a glance image used to populate the root volume.
	// Deprecated: set image in the platform spec instead. This will be
	// ignored if image is set in the platform spec.
	SourceUUID string `json:"sourceUUID,omitempty"`
	// volumeType specifies a volume type to use when creating the root
	// volume. If not specified the default volume type will be used.
	VolumeType string `json:"volumeType,omitempty"`
	// diskSize specifies the size, in GB, of the created root volume.
	Size int `json:"diskSize,omitempty"`
	// availabilityZone specifies the Cinder availability where the root volume will be created.
	Zone string `json:"availabilityZone,omitempty"`

	// Deprecated: sourceType will be silently ignored. There is no replacement.
	DeprecatedSourceType string `json:"sourceType,omitempty"`
	// Deprecated: deviceType will be silently ignored. There is no replacement.
	DeprecatedDeviceType string `json:"deviceType,omitempty"`
}
