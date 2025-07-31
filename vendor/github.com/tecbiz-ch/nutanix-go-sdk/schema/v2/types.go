package v2

import (
	"strconv"
	"time"
)

type VirtualDiskAttach struct {
	UUID    string    `json:"uuid,omitempty"`
	VMDisks []*VMDisk `json:"vm_disks,omitempty"`
}

type VirtualDiskList struct {
	Metadata *Metadata      `json:"metadata,omitempty"`
	Entities []*VirtualDisk `json:"entities,omitempty"`
}

type Metadata struct {
	Count             int    `json:"count,omitempty"`
	EndIndex          int    `json:"end_index,omitempty"`
	FilterCriteria    string `json:"filter_criteria,omitempty"`
	GrandTotalEntries int    `json:"grand_total_entities,omitempty"`
	NextCursor        string `json:"next_cursor,omitempty"`
	Page              int    `json:"page,omitempty"`
	PreviousCursor    string `json:"previous_cursor,omitempty"`
	SearchString      string `json:"search_string,omitempty"`
	SortCriteria      string `json:"sort_criteria,omitempty"`
	StartIndex        int    `json:"start_index,omitempty"`
	TotalEntries      int    `json:"total_entities,omitempty"`
}

type VirtualDisk struct {
	VirtualDiskID         string      `json:"virtual_disk_id,omitempty"`
	UUID                  string      `json:"uuid,omitempty"`
	DeviceUUID            string      `json:"device_uuid,omitempty"`
	NutanixNfsfilePath    string      `json:"nutanix_nfsfile_path,omitempty"`
	DiskAddress           string      `json:"disk_address,omitempty"`
	AttachedVMID          string      `json:"attached_vm_id,omitempty"`
	AttachedVMUUID        string      `json:"attached_vm_uuid,omitempty"`
	AttachedVMName        string      `json:"attached_vmname,omitempty"`
	AttachedVolumeGroupID string      `json:"attached_volume_group_id,omitempty"`
	DiskCapacityInBytes   int64       `json:"disk_capacity_in_bytes,omitempty"`
	ClusterUUID           string      `json:"cluster_uuid,omitempty"`
	StorageContainerID    string      `json:"storage_container_id,omitempty"`
	StorageContainerUUID  string      `json:"storage_container_uuid,omitempty"`
	FlashModeEnabled      bool        `json:"flash_mode_enabled,omitempty"`
	Stats                 interface{} `json:"stats,omitempty"`
}

type Task struct {
	TaskUUID string `json:"task_uuid,omitempty"`
}

type SnapshotList struct {
	Metadata *Metadata       `json:"metadata,omitempty"`
	Entities []*SnapshotSpec `json:"entities,omitempty"`
}

type SnapshotCreate struct {
	Metadata *Metadata       `json:"metadata,omitempty"`
	Entities []*SnapshotSpec `json:"snapshot_specs,omitempty"`
}

type SnapshotRestore struct {
	RestoreNetworkConfiguration bool   `json:"restore_network_configuration"`
	SnapshotUUID                string `json:"snapshot_uuid"`
	UUID                        string `json:"uuid"`
}

type SnapshotSpec struct {
	UUID             string      `json:"uuid,omitempty"`
	Deleted          bool        `json:"deleted,omitempty"`
	LogicalTimestamp *jsonTime   `json:"logical_timestamp,omitempty"`
	CreatedTime      *jsonTime   `json:"created_time,omitempty"`
	GroupUUID        string      `json:"group_uuid,omitempty"`
	VMUUID           string      `json:"vm_uuid,omitempty"`
	Name             string      `json:"snapshot_name,omitempty"`
	VMCreateSpec     interface{} `json:"vm_create_spec,omitempty"`
}

type PowerState string

const (
	PowerStateON           PowerState = "ON"
	PowerStateOFF          PowerState = "OFF"
	PowerStateACPISHUTDOWN PowerState = "ACPI_SHUTDOWN"
	PowerStateACPIREBOOT   PowerState = "ACPI_REBOOT"
	PowerStateRESET        PowerState = "RESET"
)

type VMPowerStateCreate struct {
	UUID               *string     `json:"uuid,omitempty"`
	HostUUID           *string     `json:"host_uuid,omitempty"`
	Transition         *PowerState `json:"transition"`
	VMLogicalTimeStamp *time.Time  `json:"vm_logical_timestamp,omitempty"`
}

type VMDiskList struct {
	UUID    *string         `json:"uuid,omitempty"`
	VMDisks []*SnapshotSpec `json:"vm_disks,omitempty"`
}

type VMDisk struct {
	DiskAddress       *VMDiskAddress `json:"disk_address,omitempty"`
	IsCDROM           *bool          `json:"is_cdrom,omitempty"`
	IsEmpty           *bool          `json:"is_empty,omitempty"`
	IsSCSIPassThrough *bool          `json:"is_scsi_pass_through,omitempty"`
	IsThinProvisioned *bool          `json:"is_thin_provisioned,omitempty"`
	VMDiskClone       *VMDiskClone   `json:"vm_disk_clone,omitempty"`
	VMDiskCreate      *VMDiskCreate  `json:"vm_disk_create,omitempty"`
}
type VMDiskCreate struct {
	Size                 *int64  `json:"size,omitempty"`
	StorageContainerUUID *string `json:"storage_container_uuid,omitempty"`
}

type VMDiskClone struct {
	DiskAddress          *VMDiskAddress `json:"disk_address,omitempty"`
	MinimumSize          *int64         `json:"minimum_size,omitempty"`
	SnapshotGroupUUID    *string        `json:"snapshot_group_uuid,omitempty"`
	StorageContainerUUID *string        `json:"storage_container_uuid,omitempty"`
}

type VMDiskAddress struct {
	DevieBus        *string `json:"device_bus,omitempty"`
	DeviceIndex     *int64  `json:"device_index,omitempty"`
	NdfsFilepath    *string `json:"ndfs_filepath,omitempty"`
	VMDiskUUID      *string `json:"vmdisk_uuid,omitempty"`
	VolumeGroupUUID *string `json:"volume_group_uuid,omitempty"`
}

type jsonTime time.Time

func (t jsonTime) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(time.Time(t).Unix(), 10)), nil
}

func (t *jsonTime) UnmarshalJSON(s []byte) (err error) {
	ss := string(s)
	if len(ss) > 1 {
		ss = ss[0:10]
	}
	q, err := strconv.ParseInt(ss, 10, 64)

	if err != nil {
		return err
	}
	*(*time.Time)(t) = time.Unix(q, 0)
	return
}

func (t jsonTime) String() string {
	return time.Time(t).String()
}
