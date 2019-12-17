//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package executors

import (
	"encoding/xml"
	"fmt"
)

type Executor interface {
	GlusterdCheck(host string) error
	PeerProbe(exec_host, newnode string) error
	PeerDetach(exec_host, detachnode string) error
	DeviceSetup(host, device, vgid string, destroy bool) (*DeviceInfo, error)
	GetDeviceInfo(host string, dh *DeviceVgHandle) (*DeviceInfo, error)
	DeviceTeardown(host string, dh *DeviceVgHandle) error
	DeviceForget(host string, dh *DeviceVgHandle) error
	BrickCreate(host string, brick *BrickRequest) (*BrickInfo, error)
	BrickDestroy(host string, brick *BrickRequest) (bool, error)
	VolumeCreate(host string, volume *VolumeRequest) (*Volume, error)
	VolumeDestroy(host string, volume string) error
	VolumeDestroyCheck(host, volume string) error
	VolumeExpand(host string, volume *VolumeRequest) (*Volume, error)
	VolumeReplaceBrick(host string, volume string, oldBrick *BrickInfo, newBrick *BrickInfo) error
	VolumeInfo(host string, volume string) (*Volume, error)
	VolumesInfo(host string) (*VolInfo, error)
	VolumeClone(host string, vsr *VolumeCloneRequest) (*Volume, error)
	VolumeSnapshot(host string, vsr *VolumeSnapshotRequest) (*Snapshot, error)
	VolumeModify(host string, mod *VolumeModifyRequest) error
	SnapshotCloneVolume(host string, scr *SnapshotCloneRequest) (*Volume, error)
	SnapshotCloneBlockVolume(host string, scr *SnapshotCloneRequest) (*BlockVolumeInfo, error)
	SnapshotDestroy(host string, snapshot string) error
	HealInfo(host string, volume string) (*HealInfo, error)
	SetLogLevel(level string)
	BlockVolumeCreate(host string, blockVolume *BlockVolumeRequest) (*BlockVolumeInfo, error)
	BlockVolumeDestroy(host string, blockHostingVolumeName string, blockVolumeName string) error
	PVS(host string) (*PVSCommandOutput, error)
	VGS(host string) (*VGSCommandOutput, error)
	LVS(host string) (*LVSCommandOutput, error)
	GetBrickMountStatus(host string) (*BricksMountStatus, error)
	ListBlockVolumes(host string, blockhostingvolume string) ([]string, error)
}

// Enumerate durability types
type DurabilityType int

const (
	DurabilityNone DurabilityType = iota
	DurabilityReplica
	DurabilityDispersion
)

type PVSCommandOutput struct {
	PVSReport []struct {
		PVS []struct {
			PVName string `json:"pv_name"`
			VGName string `json:"vg_name"`
			PVFmt  string `json:"pv_fmt"`
			PVAttr string `json:"pv_attr"`
			PVSize string `json:"pv_size"`
			PVFree string `json:"pv_free"`
		} `json:"pv"`
	} `json:"report"`
}

type VGSCommandOutput struct {
	VGSReport []struct {
		VGS []struct {
			VGName    string `json:"vg_name"`
			PVCount   string `json:"pv_count:"`
			LVCount   string `json:"lv_count"`
			SnapCount string `json:"snap_count"`
			VGAttr    string `json:"vg_attr"`
			VGSize    string `json:"vg_size"`
			VGFree    string `json:"vg_free"`
		} `json:"vg"`
	} `json:"report"`
}
type LVSCommandOutput struct {
	LVSReport []struct {
		LVS []struct {
			LVName          string `json:"lv_name"`
			VGName          string `json:"vg_name"`
			LVAttr          string `json:"lv_attr"`
			LVSize          string `json:"lv_size"`
			PoolLV          string `json:"pool_lv"`
			Origin          string `json:"origin"`
			DataPercent     string `json:"data_percent"`
			MetaDataPercent string `json:"metadata_percent"`
			MovePV          string `json:"move_pv"`
			MirrorLog       string `json:"mirror_log"`
			CopyPercent     string `json:"copy_percent"`
			ConvertLV       string `json:"convert_lv"`
		} `json:"lv"`
	} `json:"report"`
}
type BrickMountStatus struct {
	Device       string
	MountPoint   string
	Type         string
	MountOptions string
	Mounted      bool
}

type BricksMountStatus struct {
	Statuses []BrickMountStatus
}

// Returns the size of the device
type DeviceInfo struct {
	// Size in KB
	TotalSize  uint64
	FreeSize   uint64
	UsedSize   uint64
	ExtentSize uint64

	// device identification metadata
	Meta *DeviceHandle
}

type BrickFormatType int

const (
	NormalFormat BrickFormatType = iota
	ArbiterFormat
)

// Brick description
type BrickRequest struct {
	VgId             string
	Name             string
	TpSize           uint64
	Size             uint64
	PoolMetadataSize uint64
	Gid              int64
	// Path is the brick mountpoint (named Path for symmetry with BrickInfo)
	Path string
	// lvm names
	TpName string
	LvName string
	Format BrickFormatType
}

// Returns information about the location of the brick
type BrickInfo struct {
	Path string
	Host string
}

type VolumeRequest struct {
	Bricks               []BrickInfo
	Name                 string
	Type                 DurabilityType
	GlusterVolumeOptions []string

	// Dispersion
	Data       int
	Redundancy int

	// Replica
	Replica int
	Arbiter bool
}

type VolumeCloneRequest struct {
	Volume string
	Clone  string
}

type VolumeSnapshotRequest struct {
	Volume      string
	Snapshot    string
	Description string
}

type SnapshotCloneRequest struct {
	Volume   string
	Snapshot string
}

type Snapshot struct {
	XMLName xml.Name `xml:"snapshot"`
	Name    string   `xml:"name"`
	UUID    string   `xml:"uuid"`
}

type SnapCreate struct {
	XMLName  xml.Name `xml:"snapCreate"`
	Snapshot Snapshot
}

type SnapClone struct {
	XMLName xml.Name `xml:"CloneCreate"`
	Volume  VolumeClone
}

type VolumeClone struct {
	XMLName xml.Name `xml:"volume"`
	Name    string   `xml:"name"`
	UUID    string   `xml:"uuid"`
}

type SnapDelete struct {
	XMLName   xml.Name         `xml:"snapDelete"`
	Snapshots []SnapshotStatus `xml:"snapshots"`
}

type SnapshotStatus struct {
	XMLName xml.Name `xml:"snapshot"`
	Status  string   `xml:"status"`
	Name    string   `xml:"name"`
	UUID    string   `xml:"uuid"`
}

type SnapActivate struct {
	XMLName  xml.Name `xml:"snapActivate"`
	Snapshot Snapshot
}

type SnapDeactivate struct {
	XMLName  xml.Name `xml:"snapDeactivate"`
	Snapshot Snapshot
}

type Brick struct {
	UUID      string `xml:"uuid,attr"`
	Name      string `xml:"name"`
	HostUUID  string `xml:"hostUuid"`
	IsArbiter int    `xml:"isArbiter"`
}

type Bricks struct {
	XMLName   xml.Name `xml:"bricks"`
	BrickList []Brick  `xml:"brick"`
}

type BrickHealStatus struct {
	HostUUID        string `xml:"hostUuid,attr"`
	Name            string `xml:"name"`
	Status          string `xml:"status"`
	NumberOfEntries string `xml:"numberOfEntries"`
}

type Option struct {
	Name  string `xml:"name"`
	Value string `xml:"value"`
}

type Options struct {
	XMLName    xml.Name `xml:"options"`
	OptionList []Option `xml:"option"`
}

type Volume struct {
	XMLName         xml.Name `xml:"volume"`
	VolumeName      string   `xml:"name"`
	ID              string   `xml:"id"`
	Status          int      `xml:"status"`
	StatusStr       string   `xml:"statusStr"`
	BrickCount      int      `xml:"brickCount"`
	DistCount       int      `xml:"distCount"`
	StripeCount     int      `xml:"stripeCount"`
	ReplicaCount    int      `xml:"replicaCount"`
	ArbiterCount    int      `xml:"arbiterCount"`
	DisperseCount   int      `xml:"disperseCount"`
	RedundancyCount int      `xml:"redundancyCount"`
	Type            int      `xml:"type"`
	TypeStr         string   `xml:"typeStr"`
	Transport       int      `xml:"transport"`
	Bricks          Bricks
	OptCount        int `xml:"optCount"`
	Options         Options
}

type Volumes struct {
	XMLName    xml.Name `xml:"volumes"`
	Count      int      `xml:"count"`
	VolumeList []Volume `xml:"volume"`
}

type VolInfo struct {
	XMLName xml.Name `xml:"volInfo"`
	Volumes Volumes  `xml:"volumes"`
}

type HealInfoBricks struct {
	BrickList []BrickHealStatus `xml:"brick"`
}

type HealInfo struct {
	XMLName xml.Name       `xml:"healInfo"`
	Bricks  HealInfoBricks `xml:"bricks"`
}

type BlockVolumeRequest struct {
	Name              string
	Size              int
	GlusterVolumeName string
	GlusterNode       string
	Hacount           int
	BlockHosts        []string
	Auth              bool
}

type BlockVolumeInfo struct {
	Name              string
	Size              int
	GlusterVolumeName string
	GlusterNode       string
	Hacount           int
	BlockHosts        []string
	Iqn               string
	Username          string
	Password          string
}

type VolumeDoesNotExistErr struct {
	Name string
}

func (dne *VolumeDoesNotExistErr) Error() string {
	return "Volume Does Not Exist: " + dne.Name
}

// DeviceHandle identifies a device on a node by either a UUID
// or by a list of paths. Either one of UUID or Paths must be
// populated.
type DeviceHandle struct {
	UUID  string
	Paths []string
}

// DeviceVgHandle identifies a device and the vg paired with that
// device.
type DeviceVgHandle struct {
	DeviceHandle
	VgId string
}

func SimpleDeviceVgHandle(device, vgid string) *DeviceVgHandle {
	return &DeviceVgHandle{
		DeviceHandle: DeviceHandle{
			Paths: []string{device},
		},
		VgId: vgid,
	}
}

type DeviceNotAvailableErr struct {
	OriginalError error
	Path          string
	ConnectionOk  bool
	CurrentMeta   *DeviceHandle
}

func (e *DeviceNotAvailableErr) Error() string {
	head := fmt.Sprintf("Initializing device %v failed", e.Path)
	if !e.ConnectionOk {
		return fmt.Sprintf(
			"%s (failed to check device contents): %v",
			head, e.OriginalError)
	}
	if e.CurrentMeta != nil {
		return fmt.Sprintf(
			"%s (aleady contains Physical Volume %v): %v",
			head, e.CurrentMeta.UUID, e.OriginalError)
	}
	return fmt.Sprintf(
		"%s (already initialized or contains data?): %v",
		head, e.OriginalError)
}

type VolumeModifyRequest struct {
	Name string

	// Stopped should be set if the changes may only be applied
	// while the volume is stopped
	Stopped bool

	// A new set of gluster volume options
	GlusterVolumeOptions []string
}
