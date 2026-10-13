package helpers

import "time"

const (
	// IBM PI Instance
	PIInstanceName                      = "pi_instance_name"
	PIInstanceDate                      = "pi_creation_date"
	PIInstanceSSHKeyName                = "pi_key_pair_name"
	PIInstanceImageId                   = "pi_image_id"
	PIInstanceProcessors                = "pi_processors"
	PIInstanceProcType                  = "pi_proc_type"
	PIInstanceMemory                    = "pi_memory"
	PIInstanceSystemType                = "pi_sys_type"
	PIInstanceId                        = "pi_instance_id"
	PIInstanceDiskSize                  = "pi_disk_size"
	PIInstanceStatus                    = "pi_instance_status"
	PIInstanceMinProc                   = "pi_minproc"
	PIInstanceVolumeIds                 = "pi_volume_ids"
	PIInstanceNetworkIds                = "pi_network_ids"
	PIInstancePublicNetwork             = "pi_public_network"
	PIInstanceMigratable                = "pi_migratable"
	PICloudInstanceId                   = "pi_cloud_instance_id"
	PICloudInstanceSubnetName           = "pi_cloud_instance_subnet_name"
	PIInstanceMimMem                    = "pi_minmem"
	PIInstanceMaxProc                   = "pi_maxproc"
	PIInstanceMaxMem                    = "pi_maxmem"
	PIInstanceReboot                    = "pi_reboot"
	PITenantId                          = "pi_tenant_id"
	PIVirtualCoresAssigned              = "pi_virtual_cores_assigned"
	PIVirtualCoresMax                   = "pi_virtual_cores_max"
	PIVirtualCoresMin                   = "pi_virtual_cores_min"
	PIInstancePVMNetwork                = "pi_instance_pvm_network"
	PIInstanceStorageType               = "pi_storage_type"
	PIInstanceStorageConnection         = "pi_storage_connection"
	PIInstanceStoragePool               = "pi_instance_storage_pool"
	PIInstanceStorageAffinityPool       = "pi_instance_storage_affinity_pool"
	PIInstanceLicenseRepositoryCapacity = "pi_license_repository_capacity"
	PIInstanceHealthStatus              = "pi_health_status"
	PIInstanceReplicants                = "pi_replicants"
	PIInstanceReplicationPolicy         = "pi_replication_policy"
	PIInstanceReplicationScheme         = "pi_replication_scheme"
	PIInstanceProgress                  = "pi_progress"
	PIInstanceUserData                  = "pi_user_data"
	PIInstancePinPolicy                 = "pi_pin_policy"

	// IBM PI Volume
	PIVolumeName              = "pi_volume_name"
	PIVolumeSize              = "pi_volume_size"
	PIVolumeType              = "pi_volume_type"
	PIVolumeShareable         = "pi_volume_shareable"
	PIVolumeId                = "pi_volume_id"
	PIVolumeStatus            = "pi_volume_status"
	PIVolumeWWN               = "pi_volume_wwn"
	PIVolumeDeleteOnTerminate = "pi_volume_delete_on_terminate"
	PIVolumeCreateDate        = "pi_volume_create_date"
	PIVolumeLastUpdate        = "pi_last_updated_date"
	PIVolumePool              = "pi_volume_pool"
	PIAffinityPolicy          = "pi_volume_affinity_policy"
	PIAffinityVolume          = "pi_volume_affinity"
	PIAffinityInstance        = "pi_volume_affinity_instance"
	PIAffinityDiskCount       = "pi_volume_disk_count"
	PIStoragePoolValue        = "pi_storage_pool_type"
	PIStoragePoolName         = "pi_storage_pool_name"
	PIReplicationEnabled      = "pi_replication_enabled"

	// IBM PI Snapshots
	PISnapshot         = "pi_snap_shot_id"
	PISnapshotName     = "pi_snap_shot_name"
	PISnapshotStatus   = "pi_snap_shot_status"
	PISnapshotAction   = "pi_snap_shot_action"
	PISnapshotComplete = "pi_snap_shot_complete"

	// IBM PI SAP Profile
	PISAPProfileID     = "pi_sap_profile_id"
	PISAPProfile       = "pi_sap_profile"
	PISAPProfileMemory = "pi_sap_profile_memory"
	//#nosec G101
	PISAPProfileCertified          = "pi_sap_profile_certified"
	PISAPProfileType               = "pi_sap_profile_type"
	PISAPProfileCores              = "pi_sap_profile_cores"
	PISAPProfileFamilyFilterMapKey = "pi_family_filter"
	PISAPProfilePrefixFilterMapKey = "pi_prefix_filter"

	// IBM PI Clone Volume
	PIVolumeCloneStatus  = "pi_volume_clone_status"
	PIVolumeClonePercent = "pi_volume_clone_percent"
	PIVolumeCloneFailure = "pi_volume_clone_failure"

	// IBM PI Image
	PIImageName            = "pi_image_name"
	PIImageId              = "pi_image_id"
	PIImageAccessKey       = "pi_image_access_key"
	PIImageSecretKey       = "pi_image_secret_key"
	PIImageSource          = "pi_image_source"
	PIImageBucketName      = "pi_image_bucket_name"
	PIImageBucketAccess    = "pi_image_bucket_access"
	PIImageBucketFileName  = "pi_image_bucket_file_name"
	PIImageBucketRegion    = "pi_image_bucket_region"
	PIImageStorageAffinity = "pi_image_storage_affinity"
	PIImageStoragePool     = "pi_image_storage_pool"
	PIImageStorageType     = "pi_image_storage_type"
	PIImageCopyID          = "pi_image_copy_id"
	PIImagePath            = "pi_image_path"
	PIImageOsType          = "pi_image_os_type"

	// IBM PI Key
	PIKeyName = "pi_key_name"
	PIKey     = "pi_ssh_key"
	PIKeyDate = "pi_creation_date"
	PIKeyId   = "pi_key_id"

	// IBM PI Network
	PINetworkReady          = "ready"
	PINetworkID             = "pi_networkid"
	PINetworkName           = "pi_network_name"
	PINetworkCidr           = "pi_cidr"
	PINetworkDNS            = "pi_dns"
	PINetworkType           = "pi_network_type"
	PINetworkGateway        = "pi_gateway"
	PINetworkIPAddressRange = "pi_ipaddress_range"
	PINetworkVlanId         = "pi_vlan_id"
	PINetworkProvisioning   = "build"
	PINetworkJumbo          = "pi_network_jumbo"
	PINetworkMtu            = "pi_network_mtu"
	//#nosec G101
	PINetworkAccessConfig    = "pi_network_access_config"
	PINetworkPortDescription = "pi_network_port_description"
	PINetworkPortIPAddress   = "pi_network_port_ipaddress"
	PINetworkPortMacAddress  = "pi_network_port_macaddress"
	PINetworkPortStatus      = "pi_network_port_status"
	PINetworkPortPortID      = "pi_network_port_portid"

	// IBM PI Operations
	PIInstanceOperationType       = "pi_operation"
	PIInstanceOperationProgress   = "pi_progress"
	PIInstanceOperationStatus     = "pi_status"
	PIInstanceOperationServerName = "pi_instance_name"

	// IBM PI Volume Attach
	PIVolumeAttachName                = "pi_volume_attach_name"
	PIVolumeAllowableAttachStatus     = "in-use"
	PIVolumeAttachStatus              = "status"
	PowerVolumeAttachDeleting         = "deleting"
	PowerVolumeAttachProvisioning     = "creating"
	PowerVolumeAttachProvisioningDone = "available"

	// IBM PI Instance Capture
	PIInstanceCaptureName                  = "pi_capture_name"
	PIInstanceCaptureDestination           = "pi_capture_destination"
	PIInstanceCaptureVolumeIds             = "pi_capture_volume_ids"
	PIInstanceCaptureCloudStorageImagePath = "pi_capture_storage_image_path"
	PIInstanceCaptureCloudStorageRegion    = "pi_capture_cloud_storage_region"
	PIInstanceCaptureCloudStorageAccessKey = "pi_capture_cloud_storage_access_key"
	PIInstanceCaptureCloudStorageSecretKey = "pi_capture_cloud_storage_secret_key"

	// IBM PI Cloud Connections
	PICloudConnectionName          = "pi_cloud_connection_name"
	PICloudConnectionStatus        = "pi_cloud_connection_status"
	PICloudConnectionMetered       = "pi_cloud_connection_metered"
	PICloudConnectionUserIPAddress = "pi_cloud_connection_user_ip_address"
	PICloudConnectionIBMIPAddress  = "pi_cloud_connection_ibm_ip_address"
	PICloudConnectionSpeed         = "pi_cloud_connection_speed"
	PICloudConnectionPort          = "pi_cloud_connection_port"
	PICloudConnectionGlobalRouting = "pi_cloud_connection_global_routing"
	PICloudConnectionId            = "pi_cloud_connection_id"
	//PICloudConnectionClassic          = "pi_cloud_connection_classic"
	PICloudConnectionClassicEnabled   = "pi_cloud_connection_classic_enabled"
	PICloudConnectionClassicGreCidr   = "pi_cloud_connection_gre_cidr"
	PICloudConnectionClassicGreDest   = "pi_cloud_connection_gre_destination_address"
	PICloudConnectionClassicGreSource = "pi_cloud_connection_gre_source_address"
	PICloudConnectionNetworks         = "pi_cloud_connection_networks"
	//PICloudConnectionVPC              = "pi_cloud_connection_vpc"
	PICloudConnectionVPCEnabled = "pi_cloud_connection_vpc_enabled"
	PICloudConnectionVPCCRNs    = "pi_cloud_connection_vpc_crns"
	PICloudConnectionVPCName    = "pi_cloud_connection_vpc_name"

	// IBM PI VPN Connections
	PIVPNConnectionName                = "pi_vpn_connection_name"
	PIVPNConnectionId                  = "pi_vpn_connection_id"
	PIVPNIKEPolicyId                   = "pi_ike_policy_id"
	PIVPNIPSecPolicyId                 = "pi_ipsec_policy_id"
	PIVPNConnectionLocalGatewayAddress = "pi_local_gateway_address"
	PIVPNConnectionMode                = "pi_vpn_connection_mode"
	PIVPNConnectionNetworks            = "pi_networks"
	PIVPNConnectionPeerGatewayAddress  = "pi_peer_gateway_address"
	PIVPNConnectionPeerSubnets         = "pi_peer_subnets"
	//#nosec G101
	PIVPNConnectionStatus                     = "pi_vpn_connection_status"
	PIVPNConnectionVpnGatewayAddress          = "pi_gateway_address"
	PIVPNConnectionDeadPeerDetection          = "pi_dead_peer_detections"
	PIVPNConnectionDeadPeerDetectionAction    = "pi_dead_peer_detections_action"
	PIVPNConnectionDeadPeerDetectionInterval  = "pi_dead_peer_detections_interval"
	PIVPNConnectionDeadPeerDetectionThreshold = "pi_dead_peer_detections_threshold"

	// IBM PI VPN Policy
	PIVPNPolicyId             = "pi_policy_id"
	PIVPNPolicyName           = "pi_policy_name"
	PIVPNPolicyDhGroup        = "pi_policy_dh_group"
	PIVPNPolicyEncryption     = "pi_policy_encryption"
	PIVPNPolicyKeyLifetime    = "pi_policy_key_lifetime"
	PIVPNPolicyPresharedKey   = "pi_policy_preshared_key"
	PIVPNPolicyVersion        = "pi_policy_version"
	PIVPNPolicyAuthentication = "pi_policy_authentication"
	PIVPNPolicyPFS            = "pi_policy_pfs"

	JobStatusQueued             = "queued"
	JobStatusReadyForProcessing = "readyForProcessing"
	JobStatusInProgress         = "inProgress"
	JobStatusCompleted          = "completed"
	JobStatusFailed             = "failed"
	JobStatusRunning            = "running"
	JobStatusWaiting            = "waiting"

	// IBM PI DHCP
	PIDhcpId          = "pi_dhcp_id"
	PIDhcpStatus      = "pi_dhcp_status"
	PIDhcpNetwork     = "pi_dhcp_network"
	PIDhcpLeases      = "pi_dhcp_leases"
	PIDhcpInstanceIp  = "pi_dhcp_instance_ip"
	PIDhcpInstanceMac = "pi_dhcp_instance_mac"

	// IBM PI Placement Groups
	PIPlacementGroupName   = "pi_placement_group_name"
	PIPlacementGroupPolicy = "pi_placement_group_policy"
	PIPlacementGroupID     = "pi_placement_group_id"

	// Status For all the resources
	PIVolumeDeleting         = "deleting"
	PIVolumeDeleted          = "done"
	PIVolumeProvisioning     = "creating"
	PIVolumeProvisioningDone = "available"
	PIInstanceAvailable      = "ACTIVE"
	PIInstanceHealthOk       = "OK"
	PIInstanceHealthWarning  = "WARNING"
	PIInstanceBuilding       = "BUILD"
	PIInstanceDeleting       = "DELETING"
	PIInstanceNotFound       = "Not Found"
	PIImageQueStatus         = "queued"
	PIImageActiveStatus      = "active"

	// Timeout values for Power VS -
	PICreateTimeOut = 5 * time.Minute
	PIUpdateTimeOut = 5 * time.Minute
	PIDeleteTimeOut = 3 * time.Minute
	PIGetTimeOut    = 2 * time.Minute

	// Stratos region prefix
	PIStratosRegionPrefix = "satloc"

	// Standard "not supported" messages
	NotOnPremSupported  = "operation not supported in on-prem location"
	NotOffPremSupported = "operation not supported in off-prem location"
)
