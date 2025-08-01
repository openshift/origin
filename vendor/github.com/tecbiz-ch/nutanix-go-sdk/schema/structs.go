package schema

import (
	"time"
)

// VMVnumaConfig Indicates how VM vNUMA should be configured
type VMVnumaConfig struct {

	// Number of vNUMA nodes. 0 means vNUMA is disabled.
	NumVnumaNodes *int64 `json:"num_vnuma_nodes,omitempty"`
}

type VMSerialPort struct {
	Index       int64 `json:"index"`
	IsConnected bool  `json:"is_connected"`
}

// IPAddress An IP address.
type IPAddress struct {

	// Address *string.
	IP string `json:"ip,omitempty"`

	// Address type. It can only be \"ASSIGNED\" in the spec. If no type is specified in the spec, the default type is
	// set to \"ASSIGNED\".
	Type string `json:"type,omitempty"`
}

// VMNic Virtual Machine NIC.
type VMNic struct {

	// IP endpoints for the adapter. Currently, IPv4 addresses are supported.
	IPEndpointList []*IPAddress `json:"ip_endpoint_list,omitempty"`

	// The MAC address for the adapter.
	MacAddress string `json:"mac_address,omitempty"`

	// The model of this NIC.
	Model string `json:"model,omitempty"`

	NetworkFunctionChainReference *Reference `json:"network_function_chain_reference,omitempty"`

	// The type of this Network function NIC. Defaults to INGRESS.
	NetworkFunctionNicType string `json:"network_function_nic_type,omitempty"`

	// The type of this NIC. Defaults to NORMAL_NIC.
	NicType string `json:"nic_type,omitempty"`

	SubnetReference *Reference `json:"subnet_reference,omitempty"`

	// The NIC's UUID, which is used to uniquely identify this particular NIC. This UUID may be used to refer to the NIC
	// outside the context of the particular VM it is attached to.
	UUID string `json:"uuid,omitempty"`

	IsConnected bool `json:"is_connected,omitempty"`
}

// DiskAddress Disk Address.
type DiskAddress struct {
	AdapterType string `json:"adapter_type"`
	DeviceIndex int64  `json:"device_index"`
}

// VMBootDevice Indicates which device a VM should boot from. One of disk_address or mac_address should be provided.
type VMBootDevice struct {

	// Address of disk to boot from.
	DiskAddress *DiskAddress `json:"disk_address,omitempty"`

	// MAC address of nic to boot from.
	MacAddress string `json:"mac_address,omitempty"`
}

// VMBootConfig Indicates which device a VM should boot from.
type VMBootConfig struct {

	// Indicates which device a VM should boot from. Boot device takes precdence over boot device order. If both are
	// given then specified boot device will be primary boot device and remaining devices will be assigned boot order
	// according to boot device order field.
	BootDevice *VMBootDevice `json:"boot_device,omitempty"`

	// Indicates the order of device types in which VM should try to boot from. If boot device order is not provided the
	// system will decide appropriate boot device order.
	BootDeviceOrderList []string `json:"boot_device_order_list,omitempty"`

	BootType            string     `json:"boot_type,omitempty"`
	DataSourceReference *Reference `json:"data_source_reference,omitempty"`
}

// NutanixGuestToolsSpec Information regarding Nutanix Guest Tools.
type NutanixGuestToolsSpec struct {
	State                 string            `json:"state,omitempty"`                   // Nutanix Guest Tools is enabled or not.
	Version               string            `json:"version,omitempty"`                 // Version of Nutanix Guest Tools installed on the VM.
	NgtState              string            `json:"ngt_state,omitempty"`               // Nutanix Guest Tools installed or not.
	Credentials           map[string]string `json:"credentials,omitempty"`             // Credentials to login server
	IsoMountState         string            `json:"iso_mount_state,omitempty"`         // Desired mount state of Nutanix Guest Tools ISO.
	EnabledCapabilityList []string          `json:"enabled_capability_list,omitempty"` // Application names that are enabled.
}

// GuestToolsSpec Information regarding guest tools.
type GuestToolsSpec struct {

	// Nutanix Guest Tools information
	NutanixGuestTools *NutanixGuestToolsSpec `json:"nutanix_guest_tools,omitempty"`
}

// VMGpu Graphics resource information for the Virtual Machine.
type VMGpu struct {

	// The device ID of the GPU.
	DeviceID int64 `json:"device_id,omitempty"`

	// The mode of this GPU.
	Mode string `json:"mode,omitempty"`

	// The vendor of the GPU.
	Vendor string `json:"vendor,omitempty"`
}

// GuestCustomizationCloudInit If this field is set, the guest will be customized using cloud-init. Either user_data or
// custom_key_values should be provided. If custom_key_ves are provided then the user data will be generated using these
// key-value pairs.
type GuestCustomizationCloudInit struct {

	// Generic key value pair used for custom attributes
	CustomKeyValues map[string]string `json:"custom_key_values,omitempty"`

	// The contents of the meta_data configuration for cloud-init. This can be formatted as YAML or JSON. The value must
	// be base64 encoded.
	MetaData string `json:"meta_data,omitempty"`

	// The contents of the user_data configuration for cloud-init. This can be formatted as YAML, JSON, or could be a
	// shell script. The value must be base64 encoded.
	UserData string `json:"user_data,omitempty"`
}

// GuestCustomizationSysprep If this field is set, the guest will be customized using Sysprep. Either unattend_xml or
// custom_key_values should be provided. If custom_key_values are provided then the unattended answer file will be
// generated using these key-value pairs.
type GuestCustomizationSysprep struct {

	// Generic key value pair used for custom attributes
	CustomKeyValues map[string]string `json:"custom_key_values,omitempty"`

	// Whether the guest will be freshly installed using this unattend configuration, or whether this unattend
	// configuration will be applied to a pre-prepared image. Default is \"PREPARED\".
	InstallType string `json:"install_type,omitempty"`

	// This field contains a Sysprep unattend xml definition, as a *string. The value must be base64 encoded.
	UnattendXML string `json:"unattend_xml,omitempty"`
}

// GuestCustomization VM guests may be customized at boot time using one of several different methods. Currently,
// cloud-init w/ ConfigDriveV2 (for Linux VMs) and Sysprep (for Windows VMs) are supported. Only ONE OF sysprep or
// cloud_init should be provided. Note that guest customization can currently only be set during VM creation. Attempting
// to change it after creation will result in an error. Additional properties can be specified. For example - in the
// context of VM template creation if \"override_script\" is set to \"True\" then the deployer can upload their own
// custom script.
type GuestCustomization struct {
	CloudInit *GuestCustomizationCloudInit `json:"cloud_init,omitempty"`

	// Flag to allow override of customization by deployer.
	IsOverridable bool `json:"is_overridable,omitempty"`

	Sysprep *GuestCustomizationSysprep `json:"sysprep,omitempty"`
}

// VMGuestPowerStateTransitionConfig Extra configs related to power state transition.
type VMGuestPowerStateTransitionConfig struct {

	// Indicates whether to execute set script before ngt shutdown/reboot.
	EnableScriptExec bool `json:"enable_script_exec,omitempty"`

	// Indicates whether to abort ngt shutdown/reboot if script fails.
	ShouldFailOnScriptFailure bool `json:"should_fail_on_script_failure,omitempty"`
}

// VMPowerStateMechanism Indicates the mechanism guiding the VM power state transition. Currently used for the transition
// to \"OFF\" state.
type VMPowerStateMechanism struct {
	GuestTransitionConfig *VMGuestPowerStateTransitionConfig `json:"guest_transition_config,omitempty"`

	// Power state mechanism (ACPI/GUEST/HARD).
	Mechanism string `json:"mechanism,omitempty"`
}

// VMDiskDeviceProperties ...
type VMDiskDeviceProperties struct {
	DeviceType  string       `json:"device_type,omitempty"`
	DiskAddress *DiskAddress `json:"disk_address,omitempty"`
}

// VMDisk VirtualMachine Disk (VM Disk).
type VMDisk struct {
	DataSourceReference *Reference `json:"data_source_reference,omitempty"`

	DeviceProperties *VMDiskDeviceProperties `json:"device_properties,omitempty"`

	// Size of the disk in Bytes.
	DiskSizeBytes int64 `json:"disk_size_bytes,omitempty"`

	// Size of the disk in MiB. Must match the size specified in 'disk_size_bytes' - rounded up to the nearest MiB -
	// when that field is present.
	DiskSizeMib int64 `json:"disk_size_mib,omitempty"`

	// The device ID which is used to uniquely identify this particular disk.
	UUID string `json:"uuid,omitempty"`

	VolumeGroupReference *Reference `json:"volume_group_reference,omitempty"`
}

// VMResources VM Resources Definition.
type VMResources struct {

	// Indicates which device the VM should boot from.
	BootConfig *VMBootConfig `json:"boot_config,omitempty"`

	// Disks attached to the VM.
	DiskList []*VMDisk `json:"disk_list,omitempty"`

	// GPUs attached to the VM.
	GpuList []*VMGpu `json:"gpu_list,omitempty"`

	GuestCustomization *GuestCustomization `json:"guest_customization,omitempty"`

	// Guest OS Identifier. For ESX, refer to VMware documentation link
	// https://www.vmware.com/support/orchestrator/doc/vro-vsphere65-api/html/VcVirtualMachineGuestOsIdentifier.html
	// for the list of guest OS identifiers.
	GuestOsID string `json:"guest_os_id,omitempty"`

	// Information regarding guest tools.
	GuestTools *GuestToolsSpec `json:"guest_tools,omitempty"`

	// VM's hardware clock timezone in IANA TZDB format (America/Los_Angeles).
	HardwareClockTimezone string `json:"hardware_clock_timezone,omitempty"`

	// Memory size in MiB.
	MemorySizeMib int64 `json:"memory_size_mib,omitempty"`

	// NICs attached to the VM.
	NicList []*VMNic `json:"nic_list,omitempty"`

	// Number of vCPU sockets.
	NumSockets int64 `json:"num_sockets,omitempty"`

	// Number of vCPUs per socket.
	NumVcpusPerSocket int64 `json:"num_vcpus_per_socket,omitempty"`

	// *Reference to an entity that the VM should be cloned from.
	ParentReference *Reference `json:"parent_reference,omitempty"`

	// The current or desired power state of the VM.
	PowerState string `json:"power_state,omitempty"`

	PowerStateMechanism *VMPowerStateMechanism `json:"power_state_mechanism,omitempty"`

	// Indicates whether VGA console should be enabled or not.
	VgaConsoleEnabled bool `json:"vga_console_enabled,omitempty"`

	// Information regarding vNUMA configuration.
	VMVnumaConfig *VMVnumaConfig `json:"vnuma_config,omitempty"`

	SerialPortList []*VMSerialPort `json:"serial_port_list,omitempty"`
}

// VM An intentful representation of a vm spec
type VM struct {
	AvailabilityZoneReference *Reference `json:"availability_zone_reference,omitempty"`

	ClusterReference *Reference `json:"cluster_reference,omitempty"`

	// A description for vm.
	Description string `json:"description,omitempty"`

	// vm Name.
	Name string `json:"name"`

	Resources *VMResources `json:"resources,omitempty"`
}

// VMIntentInput ...
type VMIntentInput struct {
	APIVersion string `json:"api_version,omitempty"`

	Metadata *Metadata `json:"metadata"`

	Spec *VM `json:"spec"`
}

// VMStatus The status of a REST API call. Only used when there is a failure to report.
type VMStatus struct {
	APIVersion string `json:"api_version,omitempty"`

	// The HTTP error code.
	Code int64 `json:"code,omitempty"`

	// The kind name
	Kind string `json:"kind,omitempty"`

	MessageList []*MessageResource `json:"message_list,omitempty"`

	State string `json:"state,omitempty"`
}

// VMNicOutputStatus Virtual Machine NIC Status.
type VMNicOutputStatus struct {

	// The Floating IP associated with the vnic.
	FloatingIP string `json:"floating_ip,omitempty"`

	// IP endpoints for the adapter. Currently, IPv4 addresses are supported.
	IPEndpointList []*IPAddress `json:"ip_endpoint_list,omitempty"`

	// The MAC address for the adapter.
	MacAddress string `json:"mac_address,omitempty"`

	// The model of this NIC.
	Model string `json:"model,omitempty"`

	NetworkFunctionChainReference *Reference `json:"network_function_chain_reference,omitempty"`

	// The type of this Network function NIC. Defaults to INGRESS.
	NetworkFunctionNicType string `json:"network_function_nic_type,omitempty"`

	// The type of this NIC. Defaults to NORMAL_NIC.
	NicType string `json:"nic_type,omitempty"`

	SubnetReference *Reference `json:"subnet_reference,omitempty"`

	// The NIC's UUID, which is used to uniquely identify this particular NIC. This UUID may be used to refer to the NIC
	// outside the context of the particular VM it is attached to.
	UUID string `json:"uuid,omitempty"`

	IsConnected bool `json:"is_connected,omitempty"`
}

// NutanixGuestToolsStatus Information regarding Nutanix Guest Tools.
type NutanixGuestToolsStatus struct {
	// Version of Nutanix Guest Tools available on the cluster.
	AvailableVersion string `json:"available_version,omitempty"`
	// Nutanix Guest Tools installed or not.
	NgtState string `json:"ngt_state,omitempty"`
	// Desired mount state of Nutanix Guest Tools ISO.
	IsoMountState string `json:"iso_mount_state,omitempty"`
	// Nutanix Guest Tools is enabled or not.
	State string `json:"state,omitempty"`
	// Version of Nutanix Guest Tools installed on the VM.
	Version string `json:"version,omitempty"`
	// Application names that are enabled.
	EnabledCapabilityList []string `json:"enabled_capability_list,omitempty"`
	// Credentials to login server
	Credentials map[string]string `json:"credentials,omitempty"`
	// Version of the operating system on the VM.
	GuestOsVersion string `json:"guest_os_version,omitempty"`
	// Whether the VM is configured to take VSS snapshots through NGT.
	VSSSnapshotCapable bool `json:"vss_snapshot_capable,omitempty"`
	// Communication from VM to CVM is active or not.
	IsReachable bool `json:"is_reachable,omitempty"`
	// Whether VM mobility drivers are installed in the VM.
	VMMobilityDriversInstalled bool `json:"vm_mobility_drivers_installed,omitempty"`
}

// GuestToolsStatus Information regarding guest tools.
type GuestToolsStatus struct {

	// Nutanix Guest Tools information
	NutanixGuestTools *NutanixGuestToolsStatus `json:"nutanix_guest_tools,omitempty"`
}

// VMGpuOutputStatus Graphics resource status information for the Virtual Machine.
type VMGpuOutputStatus struct {

	// The device ID of the GPU.
	DeviceID int64 `json:"device_id,omitempty"`

	// Fraction of the physical GPU assigned.
	Fraction int64 `json:"fraction,omitempty"`

	// GPU frame buffer size in MiB.
	FrameBufferSizeMib int64 `json:"frame_buffer_size_mib,omitempty"`

	// Last determined guest driver version.
	GuestDriverVersion string `json:"guest_driver_version,omitempty"`

	// The mode of this GPU
	Mode string `json:"mode,omitempty"`

	// Name of the GPU resource.
	Name string `json:"name,omitempty"`

	// Number of supported virtual display heads.
	NumVirtualDisplayHeads int64 `json:"num_virtual_display_heads,omitempty"`

	// GPU {segment:bus:device:function} (sbdf) address if assigned.
	PCIAddress string `json:"pci_address,omitempty"`

	// UUID of the GPU.
	UUID string `json:"uuid,omitempty"`

	// The vendor of the GPU.
	Vendor string `json:"vendor,omitempty"`
}

// GuestCustomizationStatus VM guests may be customized at boot time using one of several different methods. Currently,
// cloud-init w/ ConfigDriveV2 (for Linux VMs) and Sysprep (for Windows VMs) are supported. Only ONE OF sysprep or
// cloud_init should be provided. Note that guest customization can currently only be set during VM creation. Attempting
// to change it after creation will result in an error. Additional properties can be specified. For example - in the
// context of VM template creation if \"override_script\" is set to \"True\" then the deployer can upload their own
// custom script.
type GuestCustomizationStatus struct {
	CloudInit *GuestCustomizationCloudInit `json:"cloud_init,omitempty"`

	// Flag to allow override of customization by deployer.
	IsOverridable bool `json:"is_overridable,omitempty"`

	Sysprep *GuestCustomizationSysprep `json:"sysprep,omitempty"`
}

// VMResourcesDefStatus VM Resources Status Definition.
type VMResourcesDefStatus struct {

	// Indicates which device the VM should boot from.
	BootConfig *VMBootConfig `json:"boot_config,omitempty"`

	// Disks attached to the VM.
	DiskList []*VMDisk `json:"disk_list,omitempty"`

	// GPUs attached to the VM.
	GpuList []*VMGpuOutputStatus `json:"gpu_list,omitempty"`

	GuestCustomization *GuestCustomizationStatus `json:"guest_customization,omitempty"`

	// Guest OS Identifier. For ESX, refer to VMware documentation link
	// https://www.vmware.com/support/orchestrator/doc/vro-vsphere65-api/html/VcVirtualMachineGuestOsIdentifier.html
	// for the list of guest OS identifiers.
	GuestOsID *string `json:"guest_os_id,omitempty"`

	// Information regarding guest tools.
	GuestTools *GuestToolsStatus `json:"guest_tools,omitempty"`

	// VM's hardware clock timezone in IANA TZDB format (America/Los_Angeles).
	HardwareClockTimezone *string `json:"hardware_clock_timezone,omitempty"`

	HostReference *Reference `json:"host_reference,omitempty"`

	// The hypervisor type for the hypervisor the VM is hosted on.
	HypervisorType *string `json:"hypervisor_type,omitempty"`

	// Memory size in MiB.
	MemorySizeMib *int64 `json:"memory_size_mib,omitempty"`

	// NICs attached to the VM.
	NicList []*VMNicOutputStatus `json:"nic_list,omitempty"`

	// Number of vCPU sockets.
	NumSockets *int64 `json:"num_sockets,omitempty"`

	// Number of vCPUs per socket.
	NumVcpusPerSocket *int64 `json:"num_vcpus_per_socket,omitempty"`

	// *Reference to an entity that the VM cloned from.
	ParentReference *Reference `json:"parent_reference,omitempty"`

	// Current power state of the VM.
	PowerState *string `json:"power_state,omitempty"`

	PowerStateMechanism *VMPowerStateMechanism `json:"power_state_mechanism,omitempty"`

	// Indicates whether VGA console has been enabled or not.
	VgaConsoleEnabled *bool `json:"vga_console_enabled,omitempty"`

	// Information regarding vNUMA configuration.
	VnumaConfig *VMVnumaConfig `json:"vnuma_config,omitempty"`

	SerialPortList []*VMSerialPort `json:"serial_port_list,omitempty"`
}

// VMDefStatus An intentful representation of a vm status
type VMDefStatus struct {
	AvailabilityZoneReference *Reference `json:"availability_zone_reference,omitempty"`

	ClusterReference *Reference `json:"cluster_reference,omitempty"`

	// A description for vm.
	Description *string `json:"description,omitempty"`

	// Any error messages for the vm, if in an error state.
	MessageList []*MessageResource `json:"message_list,omitempty"`

	// vm Name.
	Name *string `json:"name,omitempty"`

	Resources *VMResourcesDefStatus `json:"resources,omitempty"`

	// The state of the vm.
	State *string `json:"state,omitempty"`

	ExecutionContext *ExecutionContext `json:"execution_context,omitempty"`
}

// VMIntent Response object for intentful operations on a vm
type VMIntent struct {
	APIVersion string `json:"api_version,omitempty"`

	Metadata *Metadata `json:"metadata,omitempty"`

	Spec *VM `json:"spec,omitempty"`

	Status *VMDefStatus `json:"status,omitempty"`
}

type VMIntentRequest struct {
	APIVersion *string `json:"api_version,omitempty"`

	Metadata *Metadata `json:"metadata,omitempty"`

	Spec *VM `json:"spec,omitempty"`
}

type VMCloneRequest struct {
	Metadata *VMCloneMetadata `json:"metadata,omitempty"`
}

type VMCloneMetadata struct {

	// uuid
	// Pattern: ^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$
	UUID string `json:"uuid,omitempty"`
}

// VMListIntent Response object for intentful operation of vms
type VMListIntent struct {
	APIVersion string `json:"api_version,omitempty"`

	Entities []*VMIntent `json:"entities,omitempty"`

	Metadata *ListMetadata `json:"metadata,omitempty"`
}

type ListHelper struct {
	APIVersion string `json:"api_version,omitempty"`

	Entities interface{} `json:"entities,omitempty"`

	Metadata *ListMetadata `json:"metadata,omitempty"`
}

type VMIntentResource struct {
	APIVersion *string `json:"api_version,omitempty"`

	Metadata *Metadata `json:"metadata"`

	Spec *VM `json:"spec,omitempty"`

	Status *VMDefStatus `json:"status,omitempty"`
}

// PortRange represents Range of TCP/UDP ports.
type PortRange struct {
	EndPort int64 `json:"end_port,omitempty"`

	StartPort int64 `json:"start_port,omitempty"`
}

// IPSubnet IP subnet provided as an address and prefix length.
type IPSubnet struct {

	// IPV4 address.
	IP string `json:"ip,omitempty"`

	PrefixLength int64 `json:"prefix_length,omitempty"`
}

// NetworkRuleIcmpTypeCodeList ..
type NetworkRuleIcmpTypeCodeList struct {
	Code *int64 `json:"code,omitempty"`

	Type *int64 `json:"type,omitempty"`
}

// NetworkRule ...
type NetworkRule struct {

	// Timestamp of expiration time.
	ExpirationTime *string `json:"expiration_time,omitempty"`

	// The set of categories that matching VMs need to have.
	Filter *CategoryFilter `json:"filter,omitempty"`

	// List of ICMP types and codes allowed by this rule.
	IcmpTypeCodeList []*NetworkRuleIcmpTypeCodeList `json:"icmp_type_code_list,omitempty"`

	IPSubnet *IPSubnet `json:"ip_subnet,omitempty"`

	NetworkFunctionChainReference *Reference `json:"network_function_chain_reference,omitempty"`

	// The set of categories that matching VMs need to have.
	PeerSpecificationType *string `json:"peer_specification_type,omitempty"`

	// Select a protocol to allow.  Multiple protocols can be allowed by repeating network_rule object.  If a protocol
	// is not configured in the network_rule object then it is allowed.
	Protocol *string `json:"protocol,omitempty"`

	// List of TCP ports that are allowed by this rule.
	TCPPortRangeList []*PortRange `json:"tcp_port_range_list,omitempty"`

	// List of UDP ports that are allowed by this rule.
	UDPPortRangeList []*PortRange `json:"udp_port_range_list,omitempty"`
}

// TargetGroup ...
type TargetGroup struct {

	// Default policy for communication within target group.
	DefaultInternalPolicy *string `json:"default_internal_policy,omitempty"`

	// The set of categories that matching VMs need to have.
	Filter *CategoryFilter `json:"filter,omitempty"`

	// Way to identify the object for which rule is applied.
	PeerSpecificationType *string `json:"peer_specification_type,omitempty"`
}

// NetworkSecurityRuleResourcesRule These rules are used for quarantining suspected VMs. Target group is a required
// attribute.  Empty inbound_allow_list will not allow anything into target group. Empty outbound_allow_list will allow
// everything from target group.
type NetworkSecurityRuleResourcesRule struct {
	Action            *string        `json:"action,omitempty"`             // Type of action.
	InboundAllowList  []*NetworkRule `json:"inbound_allow_list,omitempty"` //
	OutboundAllowList []*NetworkRule `json:"outbound_allow_list,omitempty"`
	TargetGroup       *TargetGroup   `json:"target_group,omitempty"`
}

// NetworkSecurityRuleIsolationRule These rules are used for environmental isolation.
type NetworkSecurityRuleIsolationRule struct {
	Action             *string         `json:"action,omitempty"`               // Type of action.
	FirstEntityFilter  *CategoryFilter `json:"first_entity_filter,omitempty"`  // The set of categories that matching VMs need to have.
	SecondEntityFilter *CategoryFilter `json:"second_entity_filter,omitempty"` // The set of categories that matching VMs need to have.
}

// NetworkSecurityRuleResources ...
type NetworkSecurityRuleResources struct {
	AppRule        *NetworkSecurityRuleResourcesRule `json:"app_rule,omitempty"`
	IsolationRule  *NetworkSecurityRuleIsolationRule `json:"isolation_rule,omitempty"`
	QuarantineRule *NetworkSecurityRuleResourcesRule `json:"quarantine_rule,omitempty"`
}

// NetworkSecurityRule ...
type NetworkSecurityRule struct {
	Description *string                       `json:"description"`
	Name        *string                       `json:"name,omitempty"`
	Resources   *NetworkSecurityRuleResources `json:"resources,omitempty" `
}

// NetworkSecurityRuleIntentInput An intentful representation of a network_security_rule
type NetworkSecurityRuleIntentInput struct {
	APIVersion *string              `json:"api_version,omitempty"`
	Metadata   *Metadata            `json:"metadata"`
	Spec       *NetworkSecurityRule `json:"spec"`
}

// NetworkSecurityRuleDefStatus ... Network security rule status
type NetworkSecurityRuleDefStatus struct {
	Resources        *NetworkSecurityRuleResources `json:"resources,omitempty"`
	State            *string                       `json:"state,omitempty"`
	ExecutionContext *ExecutionContext             `json:"execution_context,omitempty"`
	Name             *string                       `json:"name,omitempty"`
	Description      *string                       `json:"description,omitempty"`
}

// NetworkSecurityRuleIntentResponse Response object for intentful operations on a network_security_rule
type NetworkSecurityRuleIntentResponse struct {
	APIVersion *string                       `json:"api_version,omitempty"`
	Metadata   *Metadata                     `json:"metadata"`
	Spec       *NetworkSecurityRule          `json:"spec,omitempty"`
	Status     *NetworkSecurityRuleDefStatus `json:"status,omitempty"`
}

// NetworkSecurityRuleStatus The status of a REST API call. Only used when there is a failure to report.
type NetworkSecurityRuleStatus struct {
	APIVersion  *string            `json:"api_version,omitempty"` //
	Code        *int64             `json:"code,omitempty"`        // The HTTP error code.
	Kind        *string            `json:"kind,omitempty"`        // The kind name
	MessageList []*MessageResource `json:"message_list,omitempty"`
	State       *string            `json:"state,omitempty"`
}

// ListMetadata All api calls that return a list will have this metadata block as input
type ListMetadata struct {
	Filter        string `json:"filter,omitempty"`         // The filter in FIQL syntax used for the results.
	Kind          string `json:"kind,omitempty"`           // The kind name
	Length        int64  `json:"length,omitempty"`         // The number of records to retrieve relative to the offset
	Offset        int64  `json:"offset,omitempty"`         // Offset from the start of the entity list
	SortAttribute string `json:"sort_attribute,omitempty"` // The attribute to perform sort on
	SortOrder     string `json:"sort_order,omitempty"`     // The sort order in which results are returned
	TotalMatches  int64  `json:"total_matches,omitempty"`  // Total matches found
}

// NetworkSecurityRuleIntentResource ... Response object for intentful operations on a network_security_rule
type NetworkSecurityRuleIntentResource struct {
	APIVersion *string                       `json:"api_version,omitempty"`
	Metadata   *Metadata                     `json:"metadata,omitempty"`
	Spec       *NetworkSecurityRule          `json:"spec,omitempty"`
	Status     *NetworkSecurityRuleDefStatus `json:"status,omitempty"`
}

// NetworkSecurityRuleListIntentResponse Response object for intentful operation of network_security_rules
type NetworkSecurityRuleListIntentResponse struct {
	APIVersion string                               `json:"api_version"`
	Entities   []*NetworkSecurityRuleIntentResource `json:"entities,omitempty" bson:"entities,omitempty"`
	Metadata   *ListMetadata                        `json:"metadata"`
}

// VolumeGroupInput Represents the request body for create volume_grop request
type VolumeGroupInput struct {
	APIVersion *string      `json:"api_version,omitempty"` // default 3.1.0
	Metadata   *Metadata    `json:"metadata,omitempty"`    // The volume_group kind metadata.
	Spec       *VolumeGroup `json:"spec,omitempty"`        // Volume group input spec.
}

// VolumeGroup Represents volume group input spec.
type VolumeGroup struct {
	Name        *string               `json:"name"`                  // Volume Group name (required)
	Description *string               `json:"description,omitempty"` // Volume Group description.
	Resources   *VolumeGroupResources `json:"resources"`             // Volume Group resources.
}

// VolumeGroupResources Represents the volume group resources
type VolumeGroupResources struct {
	FlashMode         *string         `json:"flash_mode,omitempty"`          // Flash Mode, if enabled all disks of the VG are pinned to SSD
	FileSystemType    *string         `json:"file_system_type,omitempty"`    // File system to be used for volume
	SharingStatus     *string         `json:"sharing_status,omitempty"`      // Whether the VG can be shared across multiple iSCSI initiators
	AttachmentList    []*VMAttachment `json:"attachment_list,omitempty"`     // VMs attached to volume group.
	DiskList          []*VGDisk       `json:"disk_list,omitempty"`           // VGDisk Volume group disk specification.
	IscsiTargetPrefix *string         `json:"iscsi_target_prefix,omitempty"` // iSCSI target prefix-name.
}

// VMAttachment VMs attached to volume group.
type VMAttachment struct {
	VMReference        *Reference `json:"vm_reference"`         // Reference to a kind
	IscsiInitiatorName *string    `json:"iscsi_initiator_name"` // Name of the iSCSI initiator of the workload outside Nutanix cluster.
}

// VGDisk Volume group disk specification.
type VGDisk struct {
	VmdiskUUID           *string    `json:"vmdisk_uuid"`            // The UUID of this volume disk
	Index                *int64     `json:"index"`                  // Index of the volume disk in the group.
	DataSourceReference  *Reference `json:"data_source_reference"`  // Reference to a kind
	DiskSizeMib          *int64     `json:"disk_size_mib"`          // Size of the disk in MiB.
	StorageContainerUUID *string    `json:"storage_container_uuid"` // Container UUID on which to create the disk.
}

// VolumeGroupResponse Response object for intentful operations on a volume_group
type VolumeGroupResponse struct {
	APIVersion *string               `json:"api_version"`      //
	Metadata   *Metadata             `json:"metadata"`         // The volume_group kind metadata
	Spec       *VolumeGroup          `json:"spec,omitempty"`   // Volume group input spec.
	Status     *VolumeGroupDefStatus `json:"status,omitempty"` // Volume group configuration.
}

// VolumeGroupDefStatus  Volume group configuration.
type VolumeGroupDefStatus struct {
	State       *string               `json:"state"`        // The state of the volume group entity.
	MessageList []*MessageResource    `json:"message_list"` // Volume group message list.
	Name        *string               `json:"name"`         // Volume group name.
	Resources   *VolumeGroupResources `json:"resources"`    // Volume group resources.
	Description *string               `json:"description"`  // Volume group description.
}

// VolumeGroupListResponse Response object for intentful operation of volume_groups
type VolumeGroupListResponse struct {
	APIVersion *string                `json:"api_version"`
	Entities   []*VolumeGroupResponse `json:"entities,omitempty"`
	Metadata   *ListMetadata          `json:"metadata"`
}

type TaskListIntent struct {

	// api version
	// Required: true
	APIVersion *string `json:"api_version,omitempty"`

	// entities
	// Required: true
	Entities []*Task `json:"entities,omitempty"`

	// metadata
	// Required: true
	Metadata *ListMetadata `json:"metadata,omitempty"`
}

// Task ...
type Task struct {
	Status               *string      `json:"status,omitempty"`
	LastUpdateTime       *time.Time   `json:"last_update_time,omitempty"`
	LogicalTimestamp     *int64       `json:"logical_timestamp,omitempty"`
	EntityReferenceList  []*Reference `json:"entity_reference_list,omitempty"`
	StartTime            *time.Time   `json:"start_time,omitempty"`
	CreationTime         *time.Time   `json:"creation_time,omitempty"`
	ClusterReference     *Reference   `json:"cluster_reference,omitempty"`
	SubtaskReferenceList []*Reference `json:"subtask_reference_list,omitempty"`
	CompletionTime       *time.Time   `json:"completion_timev"`
	ProgressMessage      *string      `json:"progress_message,omitempty"`
	OperationType        *string      `json:"operation_type,omitempty"`
	PercentageComplete   *int64       `json:"percentage_complete,omitempty"`
	APIVersion           *string      `json:"api_version,omitempty"`
	UUID                 *string      `json:"uuid,omitempty"`
	ErrorDetail          *string      `json:"error_detail,omitempty"`
}

// DeleteResponse ...
type DeleteResponse struct {
	Status     *DeleteStatus `json:"status"`
	Spec       string        `json:"spec"`
	APIVersion string        `json:"api_version"`
	Metadata   *Metadata     `json:"metadata"`
}

// DeleteStatus ...
type DeleteStatus struct {
	State            string            `json:"state"`
	ExecutionContext *ExecutionContext `json:"execution_context"`
}
