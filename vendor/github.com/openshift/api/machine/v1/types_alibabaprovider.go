/*
Copyright 2021 The Kubernetes Authors.

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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AlibabaDiskPerformanceLevel enum attribute to describe a disk's performance level
type AlibabaDiskPerformanceLevel string

// AlibabaDiskCatagory enum attribute to deescribe a disk's category
type AlibabaDiskCategory string

// AlibabaDiskEncryptionMode enum attribute to describe whether to enable or disable disk encryption
type AlibabaDiskEncryptionMode string

// AlibabaDiskPreservationPolicy enum attribute to describe whether to preserve or delete a disk upon instance removal
type AlibabaDiskPreservationPolicy string

// AlibabaResourceReferenceType enum attribute to identify the type of resource reference
type AlibabaResourceReferenceType string

const (
	// DeleteWithInstance enum property to delete disk with instance deletion
	DeleteWithInstance AlibabaDiskPreservationPolicy = "DeleteWithInstance"
	// PreserveDisk enum property to determine disk preservation with instance deletion
	PreserveDisk AlibabaDiskPreservationPolicy = "PreserveDisk"

	// AlibabaDiskEncryptionEnabled enum property to enable disk encryption
	AlibabaDiskEncryptionEnabled AlibabaDiskEncryptionMode = "encrypted"
	// AlibabaDiskEncryptionDisabled enum property to disable disk encryption
	AlibabaDiskEncryptionDisabled AlibabaDiskEncryptionMode = "disabled"

	// AlibabaDiskPerformanceLevel0 enum property to set the level at PL0
	PL0 AlibabaDiskPerformanceLevel = "PL0"
	// AlibabaDiskPerformanceLevel1 enum property to set the level at PL1
	PL1 AlibabaDiskPerformanceLevel = "PL1"
	// AlibabaDiskPerformanceLevel2 enum property to set the level at PL2
	PL2 AlibabaDiskPerformanceLevel = "PL2"
	// AlibabaDiskPerformanceLevel3 enum property to set the level at PL3
	PL3 AlibabaDiskPerformanceLevel = "PL3"

	// AlibabaDiskCategoryUltraDisk enum proprty to set the category of disk to ultra disk
	AlibabaDiskCatagoryUltraDisk AlibabaDiskCategory = "cloud_efficiency"
	// AlibabaDiskCategorySSD enum proprty to set the category of disk to standard SSD
	AlibabaDiskCatagorySSD AlibabaDiskCategory = "cloud_ssd"
	// AlibabaDiskCategoryESSD enum proprty to set the category of disk to ESSD
	AlibabaDiskCatagoryESSD AlibabaDiskCategory = "cloud_essd"
	// AlibabaDiskCategoryBasic enum proprty to set the category of disk to basic
	AlibabaDiskCatagoryBasic AlibabaDiskCategory = "cloud"

	// AlibabaResourceReferenceTypeID enum property to identify an ID type resource reference
	AlibabaResourceReferenceTypeID AlibabaResourceReferenceType = "ID"
	// AlibabaResourceReferenceTypeName enum property to identify an Name type resource reference
	AlibabaResourceReferenceTypeName AlibabaResourceReferenceType = "Name"
	// AlibabaResourceReferenceTypeTags enum property to identify a tags type resource reference
	AlibabaResourceReferenceTypeTags AlibabaResourceReferenceType = "Tags"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AlibabaCloudMachineProviderConfig is the Schema for the alibabacloudmachineproviderconfig API
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +k8s:openapi-gen=true
type AlibabaCloudMachineProviderConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// More detail about alibabacloud ECS
	// https://www.alibabacloud.com/help/doc-detail/25499.htm?spm=a2c63.l28256.b99.727.496d7453jF7Moz

	//The instance type of the instance.
	InstanceType string `json:"instanceType"`

	// The ID of the vpc
	VpcID string `json:"vpcId"`

	// The ID of the region in which to create the instance. You can call the DescribeRegions operation to query the most recent region list.
	RegionID string `json:"regionId"`

	// The ID of the zone in which to create the instance. You can call the DescribeZones operation to query the most recent region list.
	ZoneID string `json:"zoneId"`

	// The ID of the image used to create the instance.
	ImageID string `json:"imageId"`

	// DataDisks holds information regarding the extra disks attached to the instance
	// +optional
	DataDisks []DataDiskProperties `json:"dataDisk,omitempty"`

	// securityGroups is a list of security group references to assign to the instance.
	// A reference holds either the security group ID, the resource name, or the required tags to search.
	// When more than one security group is returned for a tag search, all the groups are associated with the instance up to the
	// maximum number of security groups to which an instance can belong.
	// For more information, see the "Security group limits" section in Limits.
	// https://www.alibabacloud.com/help/en/doc-detail/25412.htm
	SecurityGroups []AlibabaResourceReference `json:"securityGroups,omitempty"`

	// bandwidth describes the internet bandwidth strategy for the instance
	// +optional
	Bandwidth BandwidthProperties `json:"bandwidth,omitempty"`

	// systemDisk holds the properties regarding the system disk for the instance
	// +optional
	SystemDisk SystemDiskProperties `json:"systemDisk,omitempty"`

	// vSwitch is a reference to the vswitch to use for this instance.
	// A reference holds either the vSwitch ID, the resource name, or the required tags to search.
	// When more than one vSwitch is returned for a tag search, only the first vSwitch returned will be used.
	// This parameter is required when you create an instance of the VPC type.
	// You can call the DescribeVSwitches operation to query the created vSwitches.
	VSwitch AlibabaResourceReference `json:"vSwitch"`

	// ramRoleName is the name of the instance Resource Access Management (RAM) role. This allows the instance to perform API calls as this specified RAM role.
	// +optional
	RAMRoleName string `json:"ramRoleName,omitempty"`

	// resourceGroup references the resource group to which to assign the instance.
	// A reference holds either the resource group ID, the resource name, or the required tags to search.
	// When more than one resource group are returned for a search, an error will be produced and the Machine will not be created.
	// Resource Groups do not support searching by tags.
	ResourceGroup AlibabaResourceReference `json:"resourceGroup"`

	// tenancy specifies whether to create the instance on a dedicated host.
	// Valid values:
	//
	// default: creates the instance on a non-dedicated host.
	// host: creates the instance on a dedicated host. If you do not specify the DedicatedHostID parameter, Alibaba Cloud automatically selects a dedicated host for the instance.
	// Empty value means no opinion and the platform chooses the a default, which is subject to change over time.
	// Currently the default is `default`.
	// +optional
	Tenancy InstanceTenancy `json:"tenancy,omitempty"`

	// userDataSecret contains a local reference to a secret that contains the
	// UserData to apply to the instance
	// +optional
	UserDataSecret *corev1.LocalObjectReference `json:"userDataSecret,omitempty"`

	// credentialsSecret is a reference to the secret with alibabacloud credentials. Otherwise, defaults to permissions
	// provided by attached RAM role where the actuator is running.
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`

	// Tags are the set of metadata to add to an instance.
	// +optional
	Tags []Tag `json:"tag,omitempty"`
}

// ResourceTagReference is a reference to a specific AlibabaCloud resource by ID, or tags.
// Only one of ID or Tags may be specified. Specifying more than one will result in
// a validation error.
type AlibabaResourceReference struct {
	// type identifies the resource reference type for this entry.
	Type AlibabaResourceReferenceType `json:"type"`

	// id of resource
	// +optional
	ID *string `json:"id,omitempty"`

	// name of the resource
	// +optional
	Name *string `json:"name,omitempty"`

	// tags is a set of metadata based upon ECS object tags used to identify a resource.
	// For details about usage when multiple resources are found, please see the owning parent field documentation.
	// +optional
	Tags *[]Tag `json:"tags,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AlibabaCloudMachineProviderConfigList contains a list of AlibabaCloudMachineProviderConfig
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
type AlibabaCloudMachineProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AlibabaCloudMachineProviderConfig `json:"items"`
}

// AlibabaCloudMachineProviderStatus is the Schema for the alibabacloudmachineproviderconfig API
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AlibabaCloudMachineProviderStatus struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// instanceId is the instance ID of the machine created in alibabacloud
	// +optional
	InstanceID *string `json:"instanceId,omitempty"`

	// instanceState is the state of the alibabacloud instance for this machine
	// +optional
	InstanceState *string `json:"instanceState,omitempty"`

	// conditions is a set of conditions associated with the Machine to indicate
	// errors or other status
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// SystemDiskProperties contains the information regarding the system disk including performance, size, name, and category
type SystemDiskProperties struct {
	// category is the category of the system disk.
	// Valid values:
	// cloud_essd: ESSD. When the parameter is set to this value, you can use the SystemDisk.PerformanceLevel parameter to specify the performance level of the disk.
	// cloud_efficiency: ultra disk.
	// cloud_ssd: standard SSD.
	// cloud: basic disk.
	// Empty value means no opinion and the platform chooses the a default, which is subject to change over time.
	// Currently for non-I/O optimized instances of retired instance types, the default is `cloud`.
	// Currently for other instances, the default is `cloud_efficiency`.
	// +kubebuilder:validation:Enum="cloud_efficiency"; "cloud_ssd"; "cloud_essd"; "cloud"
	// +optional
	Category string `json:"category,omitempty"`

	// performanceLevel is the performance level of the ESSD used as the system disk.
	// Valid values:
	//
	// PL0: A single ESSD can deliver up to 10,000 random read/write IOPS.
	// PL1: A single ESSD can deliver up to 50,000 random read/write IOPS.
	// PL2: A single ESSD can deliver up to 100,000 random read/write IOPS.
	// PL3: A single ESSD can deliver up to 1,000,000 random read/write IOPS.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over time.
	// Currently the default is `PL1`.
	// For more information about ESSD performance levels, see ESSDs.
	// +kubebuilder:validation:Enum="PL0"; "PL1"; "PL2"; "PL3"
	// +optional
	PerformanceLevel string `json:"performanceLevel,omitempty"`

	// name is the name of the system disk. If the name is specified the name must be 2 to 128 characters in length. It must start with a letter and cannot start with http:// or https://. It can contain letters, digits, colons (:), underscores (_), and hyphens (-).
	// Empty value means the platform chooses a default, which is subject to change over time.
	// Currently the default is `""`.
	// +kubebuilder:validation:MaxLength=128
	// +optional
	Name string `json:"name,omitempty"`

	// size is the size of the system disk. Unit: GiB. Valid values: 20 to 500.
	// The value must be at least 20 and greater than or equal to the size of the image.
	// Empty value means the platform chooses a default, which is subject to change over time.
	// Currently the default is `40` or the size of the image depending on whichever is greater.
	// +optional
	Size int64 `json:"size,omitempty"`
}

// DataDisk contains the information regarding the datadisk attached to an instance
type DataDiskProperties struct {
	// Name is the name of data disk N. If the name is specified the name must be 2 to 128 characters in length. It must start with a letter and cannot start with http:// or https://. It can contain letters, digits, colons (:), underscores (_), and hyphens (-).
	//
	// Empty value means the platform chooses a default, which is subject to change over time.
	// Currently the default is `""`.
	// +optional
	Name string `name:"diskName,omitempty"`

	// SnapshotID is the ID of the snapshot used to create data disk N. Valid values of N: 1 to 16.
	//
	// When the DataDisk.N.SnapshotID parameter is specified, the DataDisk.N.Size parameter is ignored. The data disk is created based on the size of the specified snapshot.
	// Use snapshots created after July 15, 2013. Otherwise, an error is returned and your request is rejected.
	//
	// +optional
	SnapshotID string `name:"snapshotId,omitempty"`

	// Size of the data disk N. Valid values of N: 1 to 16. Unit: GiB. Valid values:
	//
	// Valid values when DataDisk.N.Category is set to cloud_efficiency: 20 to 32768
	// Valid values when DataDisk.N.Category is set to cloud_ssd: 20 to 32768
	// Valid values when DataDisk.N.Category is set to cloud_essd: 20 to 32768
	// Valid values when DataDisk.N.Category is set to cloud: 5 to 2000
	// The value of this parameter must be greater than or equal to the size of the snapshot specified by the SnapshotID parameter.
	// +optional
	Size int64 `name:"size,omitempty"`

	// DiskEncryption specifies whether to encrypt data disk N.
	//
	// Empty value means the platform chooses a default, which is subject to change over time.
	// Currently the default is `disabled`.
	// +kubebuilder:validation:Enum="encrypted";"disabled"
	// +optional
	DiskEncryption AlibabaDiskEncryptionMode `name:"diskEncryption,omitempty"`

	// PerformanceLevel is the performance level of the ESSD used as as data disk N.  The N value must be the same as that in DataDisk.N.Category when DataDisk.N.Category is set to cloud_essd.
	// Empty value means no opinion and the platform chooses a default, which is subject to change over time.
	// Currently the default is `PL1`.
	// Valid values:
	//
	// PL0: A single ESSD can deliver up to 10,000 random read/write IOPS.
	// PL1: A single ESSD can deliver up to 50,000 random read/write IOPS.
	// PL2: A single ESSD can deliver up to 100,000 random read/write IOPS.
	// PL3: A single ESSD can deliver up to 1,000,000 random read/write IOPS.
	// For more information about ESSD performance levels, see ESSDs.
	// +kubebuilder:validation:Enum="PL0"; "PL1"; "PL2"; "PL3"
	// +optional
	PerformanceLevel AlibabaDiskPerformanceLevel `name:"performanceLevel,omitempty"`

	// Category describes the type of data disk N.
	// Valid values:
	// cloud_efficiency: ultra disk
	// cloud_ssd: standard SSD
	// cloud_essd: ESSD
	// cloud: basic disk
	// Empty value means no opinion and the platform chooses the a default, which is subject to change over time.
	// Currently for non-I/O optimized instances of retired instance types, the default is `cloud`.
	// Currently for other instances, the default is `cloud_efficiency`.
	// +kubebuilder:validation:Enum="cloud_efficiency"; "cloud_ssd"; "cloud_essd"; "cloud"
	// +optional
	Category AlibabaDiskCategory `name:"category,omitempty"`

	// KMSKeyID is the ID of the Key Management Service (KMS) key to be used by data disk N.
	// Empty value means no opinion and the platform chooses the a default, which is subject to change over time.
	// Currently the default is `""` which is interpreted as do not use KMSKey encryption.
	// +optional
	KMSKeyID string `name:"kmsKeyId,omitempty"`

	// DiskPreservation specifies whether to release data disk N along with the instance.
	// Empty value means no opinion and the platform chooses the a default, which is subject to change over time.
	// Currently the default is `DeleteWithInstance`
	// +kubebuilder:validation:Enum="DeleteWithInstance";"PreserveDisk"
	// +optional
	DiskPreservation AlibabaDiskPreservationPolicy `name:"diskPreservation,omitempty"`
}

// Tag  The tags of ECS Instance
type Tag struct {
	// Key is the name of the key pair
	Key string `name:"Key"`
	// Value is the value or data of the key pair
	Value string `name:"value"`
}

// Bandwidth describes the bandwidth strategy for the network of the instance
type BandwidthProperties struct {
	// internetMaxBandwidthIn is the maximum inbound public bandwidth. Unit: Mbit/s. Valid values:
	// When the purchased outbound public bandwidth is less than or equal to 10 Mbit/s, the valid values of this parameter are 1 to 10.
	// Currently the default is `10` when outbound bandwidth is less than or equal to 10 Mbit/s.
	// When the purchased outbound public bandwidth is greater than 10, the valid values are 1 to the InternetMaxBandwidthOut value.
	// Currently the default is the value used for `InternetMaxBandwidthOut` when outbound public bandwidth is greater than 10.
	// +optional
	InternetMaxBandwidthIn int64 `json:"internetMaxBandwidthIn,omitempty"`

	// internetMaxBandwidthOut is the maximum outbound public bandwidth. Unit: Mbit/s. Valid values: 0 to 100.
	// When a value greater than 0 is used then a public IP address is assigned to the instance.
	// Empty value means no opinion and the platform chooses the a default, which is subject to change over time.
	// Currently the default is `0`
	// +optional
	InternetMaxBandwidthOut int64 `json:"internetMaxBandwidthOut,omitempty"`
}
