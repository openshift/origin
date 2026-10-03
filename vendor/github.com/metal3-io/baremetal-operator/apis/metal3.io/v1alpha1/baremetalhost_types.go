/*

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
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NOTE: json tags are required.  Any new fields you add must have
// json tags for the fields to be serialized.

// NOTE(dhellmann): Update docs/api.md when changing these data structure.

const (
	// BareMetalHostFinalizer is the name of the finalizer added to
	// hosts to block delete operations until the physical host can be
	// deprovisioned.
	BareMetalHostFinalizer string = "baremetalhost.metal3.io"

	// PausedAnnotation is the annotation that pauses the reconciliation (triggers
	// an immediate requeue).
	PausedAnnotation = "baremetalhost.metal3.io/paused"

	// DetachedAnnotation is the annotation which stops provisioner management of the host
	// unlike in the paused case, the host status may be updated.
	DetachedAnnotation = "baremetalhost.metal3.io/detached"

	// StatusAnnotation is the annotation that keeps a copy of the Status of BMH
	// This is particularly useful when we pivot BMH. If the status
	// annotation is present and status is empty, BMO will reconstruct BMH Status
	// from the status annotation.
	StatusAnnotation = "baremetalhost.metal3.io/status"

	// RebootAnnotationPrefix is the annotation which tells the host which mode to use
	// when rebooting - hard/soft.
	RebootAnnotationPrefix = "reboot.metal3.io"

	// InspectAnnotationPrefix is used to specify if automatic introspection carried out
	// during registration of BMH is enabled or disabled.
	InspectAnnotationPrefix = "inspect.metal3.io"

	// HardwareDetailsAnnotation provides the hardware details for the host
	// in case its not already part of the host status and when introspection
	// is disabled.
	HardwareDetailsAnnotation = InspectAnnotationPrefix + "/hardwaredetails"

	// InspectAnnotationValueDisabled is a constant string="disabled"
	// This is particularly useful to check if inspect annotation is disabled
	// inspect.metal3.io=disabled.
	InspectAnnotationValueDisabled = "disabled"
)

// RootDeviceHints holds the hints for specifying the storage location
// for the root filesystem for the image.
type RootDeviceHints struct {
	// A Linux device name like "/dev/vda", or a by-path link to it like
	// "/dev/disk/by-path/pci-0000:01:00.0-scsi-0:2:0:0". The hint must match
	// the actual value exactly.
	DeviceName string `json:"deviceName,omitempty"`

	// A SCSI bus address like 0:0:0:0. The hint must match the actual
	// value exactly.
	HCTL string `json:"hctl,omitempty"`

	// A vendor-specific device identifier. The hint can be a
	// substring of the actual value.
	Model string `json:"model,omitempty"`

	// The name of the vendor or manufacturer of the device. The hint
	// can be a substring of the actual value.
	Vendor string `json:"vendor,omitempty"`

	// Device serial number. The hint must match the actual value
	// exactly.
	SerialNumber string `json:"serialNumber,omitempty"`

	// The minimum size of the device in Gigabytes.
	// +kubebuilder:validation:Minimum=0
	MinSizeGigabytes int `json:"minSizeGigabytes,omitempty"`

	// Unique storage identifier. The hint must match the actual value
	// exactly.
	WWN string `json:"wwn,omitempty"`

	// Unique storage identifier with the vendor extension
	// appended. The hint must match the actual value exactly.
	WWNWithExtension string `json:"wwnWithExtension,omitempty"`

	// Unique vendor storage identifier. The hint must match the
	// actual value exactly.
	WWNVendorExtension string `json:"wwnVendorExtension,omitempty"`

	// True if the device should use spinning media, false otherwise.
	Rotational *bool `json:"rotational,omitempty"`
}

// BootMode is the boot mode of the system.
// +kubebuilder:validation:Enum=UEFI;UEFISecureBoot;legacy
type BootMode string

// Allowed boot mode from metal3.
const (
	UEFI            BootMode = "UEFI"
	UEFISecureBoot  BootMode = "UEFISecureBoot"
	Legacy          BootMode = "legacy"
	DefaultBootMode BootMode = UEFI
)

// OperationalStatus represents the state of the host.
type OperationalStatus string

const (
	// OperationalStatusOK is the status value for when the host is
	// configured correctly and is manageable.
	OperationalStatusOK OperationalStatus = "OK"

	// OperationalStatusDiscovered is the status value for when the
	// host is only partially configured, such as when the BMC
	// address is known but the login credentials are not.
	OperationalStatusDiscovered OperationalStatus = "discovered"

	// OperationalStatusError is the status value for when the host
	// has any sort of error.
	OperationalStatusError OperationalStatus = "error"

	// OperationalStatusDelayed is the status value for when the host
	// deployment needs to be delayed to limit simultaneous hosts provisioning.
	OperationalStatusDelayed = "delayed"

	// OperationalStatusDetached is the status value when the host is
	// marked unmanaged via the detached annotation.
	OperationalStatusDetached OperationalStatus = "detached"

	// OperationalStatusServicing is the status value when the host is
	// undergoing servicing (e.g. checking firmware settings).
	OperationalStatusServicing OperationalStatus = "servicing"
)

// OperationalStatusAllowed represents the allowed values of OperationalStatus.
var OperationalStatusAllowed = []string{"", string(OperationalStatusOK), string(OperationalStatusDiscovered), string(OperationalStatusError), string(OperationalStatusDelayed), string(OperationalStatusDetached)}

// ErrorType indicates the class of problem that has caused the Host resource
// to enter an error state.
type ErrorType string

const (
	// ProvisionedRegistrationError is an error condition occurring when the controller
	// is unable to re-register an already provisioned host.
	ProvisionedRegistrationError ErrorType = "provisioned registration error"
	// RegistrationError is an error condition occurring when the
	// controller is unable to connect to the Host's baseboard management
	// controller.
	RegistrationError ErrorType = "registration error"
	// InspectionError is an error condition occurring when an attempt to
	// obtain hardware details from the Host fails.
	InspectionError ErrorType = "inspection error"
	// PreparationError is an error condition occurring when do
	// cleaning steps failed.
	PreparationError ErrorType = "preparation error"
	// ProvisioningError is an error condition occurring when the controller
	// fails to provision or deprovision the Host.
	ProvisioningError ErrorType = "provisioning error"
	// PowerManagementError is an error condition occurring when the
	// controller is unable to modify the power state of the Host.
	PowerManagementError ErrorType = "power management error"
	// DetachError is an error condition occurring when the
	// controller is unable to detatch the host from the provisioner.
	DetachError ErrorType = "detach error"
	// ServicingError is an error condition occurring when
	// service steps failed.
	ServicingError ErrorType = "servicing error"
)

// ErrorTypeAllowed represents the allowed values of ErrorType.
var ErrorTypeAllowed = []string{"", string(ProvisionedRegistrationError), string(RegistrationError), string(InspectionError), string(PreparationError), string(ProvisioningError), string(PowerManagementError)}

// ProvisioningState defines the states the provisioner will report
// the host has having.
type ProvisioningState string

const (
	// StateNone means the state is unknown.
	StateNone ProvisioningState = ""

	// StateUnmanaged means there is insufficient information available to
	// register the host.
	StateUnmanaged ProvisioningState = "unmanaged"

	// StateRegistering means we are telling the backend about the host.
	StateRegistering ProvisioningState = "registering"

	// StateMatchProfile used to mean we are assigning a profile.
	// It no longer does anything, profile matching is done on registration.
	StateMatchProfile ProvisioningState = "match profile"

	// StatePreparing means we are removing existing configuration and set new configuration to the host.
	StatePreparing ProvisioningState = "preparing"

	// StateReady is a deprecated name for StateAvailable.
	StateReady ProvisioningState = "ready"

	// StateAvailable means the host can be consumed.
	StateAvailable ProvisioningState = "available"

	// StateProvisioning means we are writing an image to the host's
	// disk(s).
	StateProvisioning ProvisioningState = "provisioning"

	// StateProvisioned means we have written an image to the host's
	// disk(s).
	StateProvisioned ProvisioningState = "provisioned"

	// StateExternallyProvisioned means something else is managing the
	// image on the host.
	StateExternallyProvisioned ProvisioningState = "externally provisioned"

	// StateDeprovisioning means we are removing an image from the
	// host's disk(s).
	StateDeprovisioning ProvisioningState = "deprovisioning"

	// StateInspecting means we are running the agent on the host to
	// learn about the hardware components available there.
	StateInspecting ProvisioningState = "inspecting"

	// StatePoweringOffBeforeDelete means we are in the process of
	// powering off the host before it's deleted.
	StatePoweringOffBeforeDelete ProvisioningState = "powering off before delete"

	// StateDeleting means we are in the process of cleaning up the host
	// ready for deletion.
	StateDeleting ProvisioningState = "deleting"
)

// BMCDetails contains the information necessary to communicate with
// the bare metal controller module on host.
type BMCDetails struct {

	// Address holds the URL for accessing the controller on the network.
	// The scheme part designates the driver to use with the host.
	Address string `json:"address"`

	// The name of the secret containing the BMC credentials (requires
	// keys "username" and "password").
	CredentialsName string `json:"credentialsName"`

	// DisableCertificateVerification disables verification of server
	// certificates when using HTTPS to connect to the BMC. This is
	// required when the server certificate is self-signed, but is
	// insecure because it allows a man-in-the-middle to intercept the
	// connection.
	DisableCertificateVerification bool `json:"disableCertificateVerification,omitempty"`
}

// HardwareRAIDVolume defines the desired configuration of volume in hardware RAID.
type HardwareRAIDVolume struct {
	// Size of the logical disk to be created in GiB. If unspecified or
	// set be 0, the maximum capacity of disk will be used for logical
	// disk.
	// +kubebuilder:validation:Minimum=0
	SizeGibibytes *int `json:"sizeGibibytes,omitempty"`

	// RAID level for the logical disk. The following levels are supported:
	// 0, 1, 2, 5, 6, 1+0, 5+0, 6+0 (drivers may support only some of them).
	// +kubebuilder:validation:Enum="0";"1";"2";"5";"6";"1+0";"5+0";"6+0"
	Level string `json:"level" required:"true"`

	// Name of the volume. Should be unique within the Node. If not
	// specified, the name will be auto-generated.
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name,omitempty"`

	// Select disks with only rotational (if set to true) or solid-state
	// (if set to false) storage. By default, any disks can be picked.
	Rotational *bool `json:"rotational,omitempty"`

	// Integer, number of physical disks to use for the logical disk.
	// Defaults to minimum number of disks required for the particular RAID
	// level.
	// +kubebuilder:validation:Minimum=1
	NumberOfPhysicalDisks *int `json:"numberOfPhysicalDisks,omitempty"`

	// The name of the RAID controller to use.
	Controller string `json:"controller,omitempty"`

	// Optional list of physical disk names to be used for the hardware RAID volumes. The disk names are interpreted
	// by the hardware RAID controller, and the format is hardware specific.
	PhysicalDisks []string `json:"physicalDisks,omitempty"`
}

// SoftwareRAIDVolume defines the desired configuration of volume in software RAID.
type SoftwareRAIDVolume struct {
	// Size of the logical disk to be created in GiB.
	// If unspecified or set be 0, the maximum capacity of disk will be used for logical disk.
	// +kubebuilder:validation:Minimum=0
	SizeGibibytes *int `json:"sizeGibibytes,omitempty"`

	// RAID level for the logical disk. The following levels are supported:
	// 0, 1 and 1+0.
	// +kubebuilder:validation:Enum="0";"1";"1+0"
	Level string `json:"level" required:"true"`

	// A list of device hints, the number of items should be greater than or equal to 2.
	// +kubebuilder:validation:MinItems=2
	PhysicalDisks []RootDeviceHints `json:"physicalDisks,omitempty"`
}

// RAIDConfig contains the configuration that are required to config RAID in Bare Metal server.
type RAIDConfig struct {
	// The list of logical disks for hardware RAID, if rootDeviceHints isn't used, first volume is root volume.
	// You can set the value of this field to `[]` to clear all the hardware RAID configurations.
	// +optional
	// +nullable
	HardwareRAIDVolumes []HardwareRAIDVolume `json:"hardwareRAIDVolumes"`

	// The list of logical disks for software RAID, if rootDeviceHints isn't used, first volume is root volume.
	// If HardwareRAIDVolumes is set this item will be invalid.
	// The number of created Software RAID devices must be 1 or 2.
	// If there is only one Software RAID device, it has to be a RAID-1.
	// If there are two, the first one has to be a RAID-1, while the RAID level for the second one can be 0, 1, or 1+0.
	// As the first RAID device will be the deployment device,
	// enforcing a RAID-1 reduces the risk of ending up with a non-booting host in case of a disk failure.
	// Software RAID will always be deleted.
	// +kubebuilder:validation:MaxItems=2
	// +optional
	// +nullable
	SoftwareRAIDVolumes []SoftwareRAIDVolume `json:"softwareRAIDVolumes"`
}

// FirmwareConfig contains the configuration that you want to configure BIOS settings in Bare metal server.
type FirmwareConfig struct {
	// Supports the virtualization of platform hardware.
	// +kubebuilder:validation:Enum=true;false
	VirtualizationEnabled *bool `json:"virtualizationEnabled,omitempty"`

	// Allows a single physical processor core to appear as several logical processors.
	// +kubebuilder:validation:Enum=true;false
	SimultaneousMultithreadingEnabled *bool `json:"simultaneousMultithreadingEnabled,omitempty"`

	// SR-IOV support enables a hypervisor to create virtual instances of a PCI-express device, potentially increasing performance.
	// +kubebuilder:validation:Enum=true;false
	SriovEnabled *bool `json:"sriovEnabled,omitempty"`
}

// BareMetalHostSpec defines the desired state of BareMetalHost.
type BareMetalHostSpec struct {
	// Important: Run "make generate manifests" to regenerate code
	// after modifying this file

	// Taints is the full, authoritative list of taints to apply to
	// the corresponding Machine. This list will overwrite any
	// modifications made to the Machine on an ongoing basis.
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`

	// How do we connect to the BMC (Baseboard Management Controller) on
	// the host?
	BMC BMCDetails `json:"bmc,omitempty"`

	// RAID configuration for bare metal server. If set, the RAID settings
	// will be applied before the host is provisioned. If not, the current
	// settings will not be modified. Only one of the sub-fields
	// hardwareRAIDVolumes and softwareRAIDVolumes can be set at the same
	// time.
	RAID *RAIDConfig `json:"raid,omitempty"`

	// Firmware (BIOS) configuration for bare metal server. If set, the
	// requested settings will be applied before the host is provisioned.
	// Only some vendor drivers support this field. An alternative is to
	// use HostFirmwareSettings resources that allow changing arbitrary
	// values and support the generic Redfish-based drivers.
	Firmware *FirmwareConfig `json:"firmware,omitempty"`

	// What is the name of the hardware profile for this host?
	// Hardware profiles are deprecated and should not be used.
	// Use the separate fields Architecture and RootDeviceHints instead.
	// Set to "empty" to prepare for the future version of the API
	// without hardware profiles.
	HardwareProfile string `json:"hardwareProfile,omitempty"`

	// Provide guidance about how to choose the device for the image
	// being provisioned. The default is currently to use /dev/sda as
	// the root device.
	RootDeviceHints *RootDeviceHints `json:"rootDeviceHints,omitempty"`

	// Select the method of initializing the hardware during boot.
	// Defaults to UEFI. Legacy boot should only be used for hardware that
	// does not support UEFI correctly. Set to UEFISecureBoot to turn
	// secure boot on automatically after provisioning.
	// +optional
	BootMode BootMode `json:"bootMode,omitempty"`

	// The MAC address of the NIC used for provisioning the host. In case
	// of network boot, this is the MAC address of the PXE booting
	// interface. The MAC address of the BMC must never be used here!
	// +kubebuilder:validation:Pattern=`[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}`
	BootMACAddress string `json:"bootMACAddress,omitempty"`

	// Should the host be powered on? If the host is currently in a stable
	// state (e.g. provisioned), its power state will be forced to match
	// this value.
	Online bool `json:"online"`

	// ConsumerRef can be used to store information about something
	// that is using a host. When it is not empty, the host is
	// considered "in use". The common use case is a link to a Machine
	// resource when the host is used by Cluster API.
	ConsumerRef *corev1.ObjectReference `json:"consumerRef,omitempty"`

	// Image holds the details of the image to be provisioned. Populating
	// the image will cause the host to start provisioning.
	Image *Image `json:"image,omitempty"`

	// UserData holds the reference to the Secret containing the user data
	// which is passed to the Config Drive and interpreted by the
	// first-boot software such as cloud-init. The format of user data is
	// specific to the first-boot software.
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// PreprovisioningNetworkDataName is the name of the Secret in the
	// local namespace containing network configuration which is passed to
	// the preprovisioning image, and to the Config Drive if not overridden
	// by specifying NetworkData.
	PreprovisioningNetworkDataName string `json:"preprovisioningNetworkDataName,omitempty"`

	// NetworkData holds the reference to the Secret containing network
	// configuration which is passed to the Config Drive and interpreted
	// by the first boot software such as cloud-init.
	NetworkData *corev1.SecretReference `json:"networkData,omitempty"`

	// MetaData holds the reference to the Secret containing host metadata
	// which is passed to the Config Drive. By default, metadata will be
	// generated for the host, so most users do not need to set this field.
	MetaData *corev1.SecretReference `json:"metaData,omitempty"`

	// Description is a human-entered text used to help identify the host.
	Description string `json:"description,omitempty"`

	// ExternallyProvisioned means something else has provisioned the
	// image running on the host, and the operator should only manage
	// the power status. This field is used for integration with already
	// provisioned hosts and when pivoting hosts between clusters. If
	// unsure, leave this field as false.
	ExternallyProvisioned bool `json:"externallyProvisioned,omitempty"`

	// When set to disabled, automated cleaning will be skipped
	// during provisioning and deprovisioning.
	// +optional
	// +kubebuilder:default:=metadata
	// +kubebuilder:validation:Optional
	AutomatedCleaningMode AutomatedCleaningMode `json:"automatedCleaningMode,omitempty"`

	// A custom deploy procedure. This is an advanced feature that allows
	// using a custom deploy step provided by a site-specific deployment
	// ramdisk. Most users will want to use "image" instead. Setting this
	// field triggers provisioning.
	// +optional
	CustomDeploy *CustomDeploy `json:"customDeploy,omitempty"`

	// CPU architecture of the host, e.g. "x86_64" or "aarch64". If unset,
	// eventually populated by inspection.
	// +optional
	Architecture string `json:"architecture,omitempty"`

	// When set to true, power off of the node will be disabled,
	// instead, a reboot will be used in place of power on/off
	// +optional
	DisablePowerOff bool `json:"disablePowerOff,omitempty"`
}

// AutomatedCleaningMode is the interface to enable/disable automated cleaning
// +kubebuilder:validation:Enum:=metadata;disabled
type AutomatedCleaningMode string

// Allowed automated cleaning modes.
const (
	CleaningModeDisabled AutomatedCleaningMode = "disabled"
	CleaningModeMetadata AutomatedCleaningMode = "metadata"
)

// ChecksumType holds the algorithm name for the checksum
// +kubebuilder:validation:Enum=md5;sha256;sha512;auto
type ChecksumType string

const (
	// MD5 checksum type.
	MD5 ChecksumType = "md5"

	// SHA256 checksum type.
	SHA256 ChecksumType = "sha256"

	// SHA512 checksum type.
	SHA512 ChecksumType = "sha512"

	// Automatically detect.
	AutoChecksum ChecksumType = "auto"
)

// Image holds the details of an image either to provisioned or that
// has been provisioned.
type Image struct {
	// URL is a location of an image to deploy.
	URL string `json:"url"`

	// Checksum is the checksum for the image. Required for all formats
	// except for "live-iso".
	Checksum string `json:"checksum,omitempty"`

	// ChecksumType is the checksum algorithm for the image, e.g md5, sha256 or sha512.
	// The special value "auto" can be used to detect the algorithm from the checksum.
	// If missing, MD5 is used. If in doubt, use "auto".
	ChecksumType ChecksumType `json:"checksumType,omitempty"`

	// Format contains the format of the image (raw, qcow2, ...).
	// When set to "live-iso", an ISO 9660 image referenced by the url will
	// be live-booted and not deployed to disk.
	// +kubebuilder:validation:Enum=raw;qcow2;vdi;vmdk;live-iso
	DiskFormat *string `json:"format,omitempty"`
}

func (image *Image) IsLiveISO() bool {
	return image != nil && image.DiskFormat != nil && *image.DiskFormat == "live-iso"
}

// Custom deploy is a description of a customized deploy process.
type CustomDeploy struct {
	// Custom deploy method name.
	// This name is specific to the deploy ramdisk used. If you don't have
	// a custom deploy ramdisk, you shouldn't use CustomDeploy.
	Method string `json:"method"`
}

// FIXME(dhellmann): We probably want some other module to own these
// data structures.

// ClockSpeed is a clock speed in MHz
// +kubebuilder:validation:Format=double
type ClockSpeed float64

// ClockSpeed multipliers.
const (
	MegaHertz ClockSpeed = 1.0
	GigaHertz            = 1000 * MegaHertz
)

// Capacity is a disk size in Bytes.
type Capacity int64

// Capacity multipliers.
const (
	Byte     Capacity = 1
	KibiByte          = Byte * 1024
	KiloByte          = Byte * 1000
	MebiByte          = KibiByte * 1024
	MegaByte          = KiloByte * 1000
	GibiByte          = MebiByte * 1024
	GigaByte          = MegaByte * 1000
	TebiByte          = GibiByte * 1024
	TeraByte          = GigaByte * 1000
)

// DiskType is a disk type, i.e. HDD, SSD, NVME.
type DiskType string

// DiskType constants.
const (
	HDD  DiskType = "HDD"
	SSD  DiskType = "SSD"
	NVME DiskType = "NVME"
)

// CPU describes one processor on the host.
type CPU struct {
	Arch           string     `json:"arch,omitempty"`
	Model          string     `json:"model,omitempty"`
	ClockMegahertz ClockSpeed `json:"clockMegahertz,omitempty"`
	Flags          []string   `json:"flags,omitempty"`
	Count          int        `json:"count,omitempty"`
}

// Storage describes one storage device (disk, SSD, etc.) on the host.
type Storage struct {
	// A Linux device name of the disk, e.g.
	// "/dev/disk/by-path/pci-0000:01:00.0-scsi-0:2:0:0". This will be a name
	// that is stable across reboots if one is available.
	Name string `json:"name,omitempty"`

	// A list of alternate Linux device names of the disk, e.g. "/dev/sda".
	// Note that this list is not exhaustive, and names may not be stable
	// across reboots.
	AlternateNames []string `json:"alternateNames,omitempty"`

	// Whether this disk represents rotational storage.
	// This field is not recommended for usage, please
	// prefer using 'Type' field instead, this field
	// will be deprecated eventually.
	Rotational bool `json:"rotational,omitempty"`

	// Device type, one of: HDD, SSD, NVME.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=HDD;SSD;NVME;
	Type DiskType `json:"type,omitempty"`

	// The size of the disk in Bytes
	SizeBytes Capacity `json:"sizeBytes,omitempty"`

	// The name of the vendor of the device
	Vendor string `json:"vendor,omitempty"`

	// Hardware model
	Model string `json:"model,omitempty"`

	// The serial number of the device
	SerialNumber string `json:"serialNumber,omitempty"`

	// The WWN of the device
	WWN string `json:"wwn,omitempty"`

	// The WWN Vendor extension of the device
	WWNVendorExtension string `json:"wwnVendorExtension,omitempty"`

	// The WWN with the extension
	WWNWithExtension string `json:"wwnWithExtension,omitempty"`

	// The SCSI location of the device
	HCTL string `json:"hctl,omitempty"`
}

// VLANID is a 12-bit 802.1Q VLAN identifier
// +kubebuilder:validation:Type=integer
// +kubebuilder:validation:Minimum=0
// +kubebuilder:validation:Maximum=4094
type VLANID int32

// VLAN represents the name and ID of a VLAN.
type VLAN struct {
	ID VLANID `json:"id,omitempty"`

	Name string `json:"name,omitempty"`
}

// NIC describes one network interface on the host.
type NIC struct {
	// The name of the network interface, e.g. "en0"
	Name string `json:"name,omitempty"`

	// The vendor and product IDs of the NIC, e.g. "0x8086 0x1572"
	Model string `json:"model,omitempty"`

	// The device MAC address
	// +kubebuilder:validation:Pattern=`[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}`
	MAC string `json:"mac,omitempty"`

	// The IP address of the interface. This will be an IPv4 or IPv6 address
	// if one is present.  If both IPv4 and IPv6 addresses are present in a
	// dual-stack environment, two nics will be output, one with each IP.
	IP string `json:"ip,omitempty"`

	// The speed of the device in Gigabits per second
	SpeedGbps int `json:"speedGbps,omitempty"`

	// The VLANs available
	VLANs []VLAN `json:"vlans,omitempty"`

	// The untagged VLAN ID
	//nolint:tagliatelle
	VLANID VLANID `json:"vlanId,omitempty"`

	// Whether the NIC is PXE Bootable
	PXE bool `json:"pxe,omitempty"`
}

// Firmware describes the firmware on the host.
type Firmware struct {
	// The BIOS for this firmware
	BIOS BIOS `json:"bios,omitempty"`
}

// BIOS describes the BIOS version on the host.
type BIOS struct {
	// The release/build date for this BIOS
	Date string `json:"date,omitempty"`

	// The vendor name for this BIOS
	Vendor string `json:"vendor,omitempty"`

	// The version of the BIOS
	Version string `json:"version,omitempty"`
}

// HardwareDetails collects all of the information about hardware
// discovered on the host.
type HardwareDetails struct {
	// System vendor information.
	SystemVendor HardwareSystemVendor `json:"systemVendor,omitempty"`
	// System firmware information.
	Firmware Firmware `json:"firmware,omitempty"`
	// The host's amount of memory in Mebibytes.
	RAMMebibytes int `json:"ramMebibytes,omitempty"`
	// List of network interfaces for the host.
	NIC []NIC `json:"nics,omitempty"`
	// List of storage (disk, SSD, etc.) available to the host.
	Storage []Storage `json:"storage,omitempty"`
	// Details of the CPU(s) in the system.
	CPU      CPU    `json:"cpu,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

// HardwareSystemVendor stores details about the whole hardware system.
type HardwareSystemVendor struct {
	Manufacturer string `json:"manufacturer,omitempty"`
	ProductName  string `json:"productName,omitempty"`
	SerialNumber string `json:"serialNumber,omitempty"`
}

// CredentialsStatus contains the reference and version of the last
// set of BMC credentials the controller was able to validate.
type CredentialsStatus struct {
	Reference *corev1.SecretReference `json:"credentials,omitempty"`
	Version   string                  `json:"credentialsVersion,omitempty"`
}

// RebootMode defines known variations of reboot modes.
type RebootMode string

const (
	// RebootModeHard defined for hard reset of a host.
	RebootModeHard RebootMode = "hard"
	// RebootModeSoft defined for soft reset of a host.
	RebootModeSoft RebootMode = "soft"
)

// RebootAnnotationArguments defines the arguments of the RebootAnnotation type.
type RebootAnnotationArguments struct {
	Mode  RebootMode `json:"mode"`
	Force bool       `json:"force"`
}

type DetachedDeleteAction string

const (
	DetachedDeleteActionDelay  = "delay"
	DetachedDeleteActionDelete = "delete"
)

type DetachedAnnotationArguments struct {
	// DeleteAction indicates the desired delete logic when the detached annotation is present
	DeleteAction DetachedDeleteAction `json:"deleteAction,omitempty"`
}

// Match compares the saved status information with the name and
// content of a secret object.
func (cs CredentialsStatus) Match(secret corev1.Secret) bool {
	switch {
	case cs.Reference == nil:
		return false
	case cs.Reference.Name != secret.ObjectMeta.Name:
		return false
	case cs.Reference.Namespace != secret.ObjectMeta.Namespace:
		return false
	case cs.Version != secret.ObjectMeta.ResourceVersion:
		return false
	}
	return true
}

// OperationMetric contains metadata about an operation (inspection,
// provisioning, etc.) used for tracking metrics.
type OperationMetric struct {
	// +nullable
	Start metav1.Time `json:"start,omitempty"`
	// +nullable
	End metav1.Time `json:"end,omitempty"`
}

// Duration returns the length of time that was spent on the
// operation. If the operation is not finished, it returns 0.
func (om OperationMetric) Duration() time.Duration {
	if om.Start.IsZero() {
		return 0
	}
	return om.End.Time.Sub(om.Start.Time)
}

// OperationHistory holds information about operations performed on a
// host.
type OperationHistory struct {
	Register    OperationMetric `json:"register,omitempty"`
	Inspect     OperationMetric `json:"inspect,omitempty"`
	Provision   OperationMetric `json:"provision,omitempty"`
	Deprovision OperationMetric `json:"deprovision,omitempty"`
}

// BareMetalHostStatus defines the observed state of BareMetalHost.
type BareMetalHostStatus struct {
	// Important: Run "make generate manifests" to regenerate code
	// after modifying this file

	// OperationalStatus holds the status of the host
	// +kubebuilder:validation:Enum="";OK;discovered;error;delayed;detached;servicing
	OperationalStatus OperationalStatus `json:"operationalStatus"`

	// ErrorType indicates the type of failure encountered when the
	// OperationalStatus is OperationalStatusError
	// +kubebuilder:validation:Enum=provisioned registration error;registration error;inspection error;preparation error;provisioning error;power management error;servicing error
	ErrorType ErrorType `json:"errorType,omitempty"`

	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// The name of the profile matching the hardware details.
	// Hardware profiles are deprecated and should not be relied on.
	HardwareProfile string `json:"hardwareProfile,omitempty"`

	// The hardware discovered to exist on the host.
	// This field will be removed in the next API version in favour of the
	// separate HardwareData resource.
	HardwareDetails *HardwareDetails `json:"hardware,omitempty"`

	// Information tracked by the provisioner.
	Provisioning ProvisionStatus `json:"provisioning"`

	// The last credentials we were able to validate as working.
	GoodCredentials CredentialsStatus `json:"goodCredentials,omitempty"`

	// The last credentials we sent to the provisioning backend.
	TriedCredentials CredentialsStatus `json:"triedCredentials,omitempty"`

	// The last error message reported by the provisioning subsystem.
	ErrorMessage string `json:"errorMessage"`

	// The currently detected power state of the host. This field may get
	// briefly out of sync with the actual state of the hardware while
	// provisioning processes are running.
	PoweredOn bool `json:"poweredOn"`

	// OperationHistory holds information about operations performed
	// on this host.
	OperationHistory OperationHistory `json:"operationHistory,omitempty"`

	// ErrorCount records how many times the host has encoutered an error since the last successful operation
	// +kubebuilder:default:=0
	ErrorCount int `json:"errorCount"`
}

// ProvisionStatus holds the state information for a single target.
type ProvisionStatus struct {
	// An indicator for what the provisioner is doing with the host.
	State ProvisioningState `json:"state"`

	// The hosts's ID from the underlying provisioning tool (e.g. the
	// Ironic node UUID).
	//nolint:tagliatelle
	ID string `json:"ID"`

	// Image holds the details of the last image successfully
	// provisioned to the host.
	Image Image `json:"image,omitempty"`

	// The root device hints used to provision the host.
	RootDeviceHints *RootDeviceHints `json:"rootDeviceHints,omitempty"`

	// BootMode indicates the boot mode used to provision the host.
	BootMode BootMode `json:"bootMode,omitempty"`

	// The RAID configuration that has been applied.
	RAID *RAIDConfig `json:"raid,omitempty"`

	// The firmware settings that have been applied.
	Firmware *FirmwareConfig `json:"firmware,omitempty"`

	// Custom deploy procedure applied to the host.
	CustomDeploy *CustomDeploy `json:"customDeploy,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BareMetalHost is the Schema for the baremetalhosts API
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName=bmh;bmhost
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.operationalStatus",description="Operational status",priority=1
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.provisioning.state",description="Provisioning status"
// +kubebuilder:printcolumn:name="Consumer",type="string",JSONPath=".spec.consumerRef.name",description="Consumer using this host"
// +kubebuilder:printcolumn:name="BMC",type="string",JSONPath=".spec.bmc.address",description="Address of management controller",priority=1
// +kubebuilder:printcolumn:name="Online",type="string",JSONPath=".spec.online",description="Whether the host is online or not"
// +kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.errorType",description="Type of the most recent error"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of BaremetalHost"
// +kubebuilder:object:root=true
type BareMetalHost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BareMetalHostSpec   `json:"spec,omitempty"`
	Status BareMetalHostStatus `json:"status,omitempty"`
}

// BootMode returns the boot method to use for the host.
func (host *BareMetalHost) BootMode() BootMode {
	mode := host.Spec.BootMode
	if mode == "" {
		return DefaultBootMode
	}
	return mode
}

// HasBMCDetails returns true if the BMC details are set.
func (host *BareMetalHost) HasBMCDetails() bool {
	return host.Spec.BMC.Address != "" || host.Spec.BMC.CredentialsName != ""
}

// NeedsHardwareProfile returns true if the profile is not set.
func (host *BareMetalHost) NeedsHardwareProfile() bool {
	return host.Status.HardwareProfile == ""
}

// HardwareProfile returns the hardware profile name for the host.
func (host *BareMetalHost) HardwareProfile() string {
	return host.Status.HardwareProfile
}

// SetHardwareProfile updates the hardware profile name and returns
// true when a change is made or false when no change is made.
func (host *BareMetalHost) SetHardwareProfile(name string) (dirty bool) {
	if host.Status.HardwareProfile != name {
		host.Status.HardwareProfile = name
		dirty = true
	}
	return dirty
}

// SetOperationalStatus updates the OperationalStatus field and returns
// true when a change is made or false when no change is made.
func (host *BareMetalHost) SetOperationalStatus(status OperationalStatus) bool {
	if host.Status.OperationalStatus != status {
		host.Status.OperationalStatus = status
		return true
	}
	return false
}

// OperationalStatus returns the contents of the OperationalStatus
// field.
func (host *BareMetalHost) OperationalStatus() OperationalStatus {
	return host.Status.OperationalStatus
}

// CredentialsKey returns a NamespacedName suitable for loading the
// Secret containing the credentials associated with the host.
func (host *BareMetalHost) CredentialsKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      host.Spec.BMC.CredentialsName,
		Namespace: host.ObjectMeta.Namespace,
	}
}

// NeedsHardwareInspection looks at the state of the host to determine
// if hardware inspection should be run.
func (host *BareMetalHost) NeedsHardwareInspection() bool {
	if host.Spec.ExternallyProvisioned {
		// Never perform inspection if we already know something is
		// using the host and we didn't provision it.
		return false
	}
	if host.WasProvisioned() {
		// Never perform inspection if we have already provisioned
		// this host, because we don't want to reboot it.
		return false
	}
	return host.Status.HardwareDetails == nil
}

// NeedsProvisioning compares the settings with the provisioning
// status and returns true when more work is needed or false
// otherwise.
func (host *BareMetalHost) NeedsProvisioning() bool {
	if !host.Spec.Online {
		// The host is not supposed to be powered on.
		return false
	}

	return host.hasNewImage() || host.hasNewCustomDeploy()
}

func (host *BareMetalHost) hasNewImage() bool {
	if host.Spec.Image == nil {
		// Without an image, there is nothing to provision.
		return false
	}
	if host.Spec.Image.URL == "" {
		// We have an Image struct but it is empty
		return false
	}
	if host.Status.Provisioning.Image.URL == "" {
		// We have an image set, but not provisioned.
		return true
	}
	return false
}

func (host *BareMetalHost) hasNewCustomDeploy() bool {
	if host.Spec.CustomDeploy == nil {
		return false
	}
	if host.Spec.CustomDeploy.Method == "" {
		return false
	}
	if host.Status.Provisioning.CustomDeploy == nil {
		return true
	}
	if host.Status.Provisioning.CustomDeploy.Method != host.Spec.CustomDeploy.Method {
		return true
	}
	return false
}

// WasProvisioned returns true when we think we have placed an image
// on the host.
func (host *BareMetalHost) WasProvisioned() bool {
	if host.Spec.ExternallyProvisioned {
		return false
	}
	if host.Status.Provisioning.Image.URL != "" {
		// We have an image provisioned.
		return true
	}
	if host.Status.Provisioning.CustomDeploy != nil {
		// We have a custom deploy provisioned.
		return true
	}
	return false
}

// UpdateGoodCredentials modifies the GoodCredentials portion of the
// Status struct to record the details of the secret containing
// credentials known to work.
func (host *BareMetalHost) UpdateGoodCredentials(currentSecret corev1.Secret) {
	host.Status.GoodCredentials.Version = currentSecret.ObjectMeta.ResourceVersion
	host.Status.GoodCredentials.Reference = &corev1.SecretReference{
		Name:      currentSecret.ObjectMeta.Name,
		Namespace: currentSecret.ObjectMeta.Namespace,
	}
}

// UpdateTriedCredentials modifies the TriedCredentials portion of the
// Status struct to record the details of the secret containing
// credentials known to work.
func (host *BareMetalHost) UpdateTriedCredentials(currentSecret corev1.Secret) {
	host.Status.TriedCredentials.Version = currentSecret.ObjectMeta.ResourceVersion
	host.Status.TriedCredentials.Reference = &corev1.SecretReference{
		Name:      currentSecret.ObjectMeta.Name,
		Namespace: currentSecret.ObjectMeta.Namespace,
	}
}

// NewEvent creates a new event associated with the object and ready
// to be published to the kubernetes API.
func (host *BareMetalHost) NewEvent(reason, message string) corev1.Event {
	t := metav1.Now()
	return corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: reason + "-",
			Namespace:    host.ObjectMeta.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:       "BareMetalHost",
			Namespace:  host.Namespace,
			Name:       host.Name,
			UID:        host.UID,
			APIVersion: GroupVersion.String(),
		},
		Reason:  reason,
		Message: message,
		Source: corev1.EventSource{
			Component: "metal3-baremetal-controller",
		},
		FirstTimestamp:      t,
		LastTimestamp:       t,
		Count:               1,
		Type:                corev1.EventTypeNormal,
		ReportingController: "metal3.io/baremetal-controller",
		Related:             host.Spec.ConsumerRef,
	}
}

// OperationMetricForState returns a pointer to the metric for the given
// provisioning state.
func (host *BareMetalHost) OperationMetricForState(operation ProvisioningState) (metric *OperationMetric) {
	history := &host.Status.OperationHistory
	switch operation {
	case StateRegistering:
		metric = &history.Register
	case StateInspecting:
		metric = &history.Inspect
	case StateProvisioning:
		metric = &history.Provision
	case StateDeprovisioning:
		metric = &history.Deprovision
	default:
	}
	return
}

var supportedChecksums = strings.Join([]string{string(AutoChecksum), string(MD5), string(SHA256), string(SHA512)}, ", ")

// GetChecksum method returns the checksum of an image.
func (image *Image) GetChecksum() (checksum, checksumType string, err error) {
	if image == nil {
		return "", "", errors.New("image is not provided")
	}

	if image.DiskFormat != nil && *image.DiskFormat == "live-iso" {
		// Checksum is not required for live-iso
		return "", "", nil
	}

	// FIXME(dtantsur): Ironic supports oci:// images with an embedded checksum
	if image.Checksum == "" {
		// Return empty if checksum is not provided
		return "", "", errors.New("checksum is required for normal images")
	}

	switch image.ChecksumType {
	case MD5, SHA256, SHA512:
		checksumType = string(image.ChecksumType)
	case "", AutoChecksum:
		// No type, let Ironic detect
	default:
		return "", "", fmt.Errorf("unknown checksumType %s, supported are %s", image.ChecksumType, supportedChecksums)
	}

	checksum = image.Checksum
	return checksum, checksumType, nil
}

// +kubebuilder:object:root=true

// BareMetalHostList contains a list of BareMetalHost.
type BareMetalHostList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalHost `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalHost{}, &BareMetalHostList{})
}
