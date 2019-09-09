//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/lpabon/godbc"

	"github.com/heketi/heketi/pkg/idgen"
)

type BrickSet struct {
	SetSize int
	Bricks  []*BrickEntry
}

// NewBrickSet creates an empty brick set tracking object with
// the expected final size of s.
func NewBrickSet(s int) *BrickSet {
	return &BrickSet{SetSize: s, Bricks: []*BrickEntry{}}
}

// NewSparseBrickSet creates a brick set tracking object with
// the expected final size of s and the brick tracking slice
// pre-sized to s. This means that the array can be directly
// indexed but may contain null brick entries.
func NewSparseBrickSet(s int) *BrickSet {
	return &BrickSet{
		SetSize: s,
		Bricks:  make([]*BrickEntry, s),
	}
}

func (bs *BrickSet) Add(b *BrickEntry) {
	godbc.Require(!bs.Full())
	bs.Bricks = append(bs.Bricks, b)
}

func (bs *BrickSet) Insert(index int, b *BrickEntry) {
	switch {
	case index >= bs.SetSize:
		panic(fmt.Errorf("Insert index (%v) out of bounds", index))
	case index == len(bs.Bricks):
		// we grow the bricks slice by one item
		bs.Bricks = append(bs.Bricks, b)
	case index < len(bs.Bricks):
		// we replace an existing item
		bs.Bricks[index] = b
	default:
		panic(fmt.Errorf(
			"Brick set may only be extended one (index=%v, len=%v)",
			index, len(bs.Bricks)))
	}
}

func (bs *BrickSet) Contents() []*BrickEntry {
	out := []*BrickEntry{}
	for _, b := range bs.Bricks {
		if b != nil {
			out = append(out, b)
		}
	}
	return out
}

// IsSparse returns true if the brick tracking slice contains
// any nulls, indicating that the BrickSet is not fully populated.
func (bs *BrickSet) IsSparse() bool {
	for _, b := range bs.Bricks {
		if b == nil {
			return true
		}
	}
	return false
}

// Full returns true if the brick slice contains the same
// number of items as the expected set size.
func (bs *BrickSet) Full() bool {
	return len(bs.Bricks) == bs.SetSize
}

// Drop returns a new brick set with the brick at the given
// index removed. Does not preserve brick positioning and
// is not suitable for position dependent allocations.
func (bs *BrickSet) Drop(index int) *BrickSet {
	bs2 := NewBrickSet(bs.SetSize)
	bs2.Bricks = append(bs.Bricks[:index], bs.Bricks[index+1:]...)
	return bs2
}

func (bs *BrickSet) String() string {
	ids := []string{}
	for _, b := range bs.Bricks {
		ids = append(ids, b.Id())
	}
	return fmt.Sprintf("BrickSet(%v)%v", bs.SetSize, ids)
}

type DeviceSet struct {
	SetSize int
	Devices []*DeviceEntry
}

// NewDeviceSet creates a device set tracking object with
// the expected final size of s.
func NewDeviceSet(s int) *DeviceSet {
	return &DeviceSet{SetSize: s, Devices: []*DeviceEntry{}}
}

// NewSparseDeviceSet creates a device set tracking object with
// the expected final size of s and the device tracking slice
// pre-sized to s. This means that the array can be directly
// indexed but may contain null brick entries.
func NewSparseDeviceSet(s int) *DeviceSet {
	return &DeviceSet{
		SetSize: s,
		Devices: make([]*DeviceEntry, s),
	}
}

func (ds *DeviceSet) Add(d *DeviceEntry) {
	godbc.Require(!ds.Full())
	ds.Devices = append(ds.Devices, d)
}

func (ds *DeviceSet) Insert(index int, d *DeviceEntry) {
	switch {
	case index >= ds.SetSize:
		panic(fmt.Errorf("Insert index (%v) out of bounds", index))
	case index == len(ds.Devices):
		// we grow the bricks slice by one item
		ds.Devices = append(ds.Devices, d)
	case index < len(ds.Devices):
		// we replace an existing item
		ds.Devices[index] = d
	default:
		panic(fmt.Errorf(
			"Brick set may only be extended one (index=%v, len=%v)",
			index, len(ds.Devices)))
	}
}

// Full returns true if the device slice contains the same
// number of items as the expected set size.
func (ds *DeviceSet) Full() bool {
	return len(ds.Devices) == ds.SetSize
}

// IsSparse returns true if the device tracking slice contains
// any nulls, indicating that the DeviceSet is not fully populated.
func (ds *DeviceSet) IsSparse() bool {
	for _, d := range ds.Devices {
		if d == nil {
			return true
		}
	}
	return false
}

type BrickAllocation struct {
	BrickSets  []*BrickSet
	DeviceSets []*DeviceSet
}

type deviceFetcher func(string) (*DeviceEntry, error)

func tryAllocateBrickOnDevice(
	opts PlacementOpts,
	pred DeviceFilter,
	device *DeviceEntry,
	bs *BrickSet) *BrickEntry {

	// Do not allow a device from the same node to be in the set
	deviceOk := true
	for _, brickInSet := range bs.Bricks {
		if brickInSet.Info.NodeId == device.NodeId {
			deviceOk = false
		}
	}

	if !deviceOk {
		return nil
	}
	if pred != nil && !pred(bs, device) {
		return nil
	}

	// Try to allocate a brick on this device
	brickSize, snapFactor := opts.BrickSizes()
	brick := device.NewBrickEntry(brickSize, snapFactor,
		opts.BrickGid(), opts.BrickOwner())
	if brick == nil {
		logger.Debug(
			"Unable to place a brick of size %v & factor %v on device %v",
			brickSize, snapFactor, device.Info.Id)
	} else {
		brick.SubType = NormalSubType
	}
	return brick
}

func findDeviceAndBrickForSet(
	opts PlacementOpts,
	fetchDevice deviceFetcher,
	pred DeviceFilter,
	deviceCh <-chan string,
	bs *BrickSet) (*BrickEntry, *DeviceEntry, error) {

	// Check the ring for devices to place the brick
	for deviceId := range deviceCh {

		device, err := fetchDevice(deviceId)
		if err != nil {
			return nil, nil, err
		}

		brick := tryAllocateBrickOnDevice(opts, pred, device, bs)
		if brick == nil {
			continue
		}

		return brick, device, nil
	}

	// No devices found
	return nil, nil, ErrNoSpace
}

func populateBrickSet(
	opts PlacementOpts,
	fetchDevice deviceFetcher,
	pred DeviceFilter,
	deviceCh <-chan string,
	initId string) (*BrickSet, *DeviceSet, error) {

	ssize := opts.SetSize()
	bs := NewBrickSet(ssize)
	ds := NewDeviceSet(ssize)
	for i := 0; i < ssize; i++ {
		logger.Debug("%v / %v", i, ssize)

		brick, device, err := findDeviceAndBrickForSet(
			opts, fetchDevice, pred, deviceCh, bs)
		if err != nil {
			return bs, ds, err
		}

		// If the first in the set, then reset the id
		if i == 0 {
			brick.SetId(initId)
		}

		// Save the brick entry to create later
		bs.Add(brick)
		ds.Add(device)

		device.BrickAdd(brick.Id())
	}
	return bs, ds, nil
}

type ClusterDeviceSource struct {
	tx          *bolt.Tx
	deviceCache map[string]*DeviceEntry
	nodeCache   map[string]*NodeEntry
	clusterId   string
}

func NewClusterDeviceSource(tx *bolt.Tx,
	clusterId string) *ClusterDeviceSource {

	return &ClusterDeviceSource{
		tx:          tx,
		deviceCache: map[string](*DeviceEntry){},
		nodeCache:   map[string](*NodeEntry){},
		clusterId:   clusterId,
	}
}

func (cds *ClusterDeviceSource) Devices() ([]DeviceAndNode, error) {
	cluster, err := NewClusterEntryFromId(cds.tx, cds.clusterId)
	if err != nil {
		return nil, err
	}

	if len(cluster.Info.Nodes) == 0 {
		return nil, ErrEmptyCluster
	}

	nodeUp := currentNodeHealthStatus()

	valid := [](DeviceAndNode){}
	for _, nodeId := range cluster.Info.Nodes {
		node, err := NewNodeEntryFromId(cds.tx, nodeId)
		if err != nil {
			return nil, err
		}
		if !node.isOnline() {
			continue
		}
		if up, found := nodeUp[nodeId]; found && !up {
			// if the node is in the cache and we know it was not
			// recently healthy, skip it
			continue
		}

		for _, deviceId := range node.Devices {
			device, err := NewDeviceEntryFromId(cds.tx, deviceId)
			if err != nil {
				return nil, err
			}
			if !device.isOnline() {
				continue
			}

			valid = append(valid, DeviceAndNode{
				Device: device,
				Node:   node,
			})
			// NOTE: it is extremely important not to overwrite
			// existing cache items because the allocation algorithms
			// mutate the device entries during the process.
			if _, found := cds.deviceCache[deviceId]; !found {
				cds.deviceCache[deviceId] = device
			}
			if _, found := cds.nodeCache[nodeId]; !found {
				cds.nodeCache[nodeId] = node
			}
		}
	}
	if len(valid) == 0 {
		return nil, ErrNoStorage
	}

	return valid, nil
}

func (cds *ClusterDeviceSource) Device(id string) (*DeviceEntry, error) {
	device, ok := cds.deviceCache[id]
	if !ok {
		// Get device entry from db otherwise
		var err error
		device, err = NewDeviceEntryFromId(cds.tx, id)
		if err != nil {
			return nil, err
		}
		cds.deviceCache[id] = device
	}
	return device, nil
}

func (cds *ClusterDeviceSource) Node(id string) (*NodeEntry, error) {
	node, ok := cds.nodeCache[id]
	if !ok {
		// Get node entry from db otherwise
		var err error
		node, err = NewNodeEntryFromId(cds.tx, id)
		if err != nil {
			return nil, err
		}
		cds.nodeCache[id] = node
	}
	return node, nil
}

type VolumePlacementOpts struct {
	v            *VolumeEntry
	brickSize    uint64
	numBrickSets int
}

func NewVolumePlacementOpts(v *VolumeEntry,
	brickSize uint64, numBrickSets int) *VolumePlacementOpts {
	return &VolumePlacementOpts{v, brickSize, numBrickSets}
}

func (vp *VolumePlacementOpts) BrickSizes() (uint64, float64) {
	return vp.brickSize, float64(vp.v.Info.Snapshot.Factor)
}

func (vp *VolumePlacementOpts) BrickOwner() string {
	return vp.v.Info.Id
}

func (vp *VolumePlacementOpts) BrickGid() int64 {
	return vp.v.Info.Gid
}

func (vp *VolumePlacementOpts) SetSize() int {
	return vp.v.Durability.BricksInSet()
}

func (vp *VolumePlacementOpts) SetCount() int {
	return vp.numBrickSets
}

func (vp *VolumePlacementOpts) AverageFileSize() uint64 {
	return vp.v.GetAverageFileSize()
}

type StandardBrickPlacer struct{}

func NewStandardBrickPlacer() *StandardBrickPlacer {
	return &StandardBrickPlacer{}
}

func (bp *StandardBrickPlacer) PlaceAll(
	dsrc DeviceSource,
	opts PlacementOpts,
	pred DeviceFilter) (
	*BrickAllocation, error) {

	r := &BrickAllocation{
		BrickSets:  []*BrickSet{},
		DeviceSets: []*DeviceSet{},
	}

	numBrickSets := opts.SetCount()
	for sn := 0; sn < numBrickSets; sn++ {
		logger.Info("Allocating brick set #%v", sn)

		// Generate an id for the brick, this is used as a
		// random index into the ring(s)
		brickId := idgen.GenUUID()

		a := NewSimpleAllocator()
		deviceCh, done, err := a.GetNodesFromDeviceSource(dsrc, brickId)
		defer close(done)
		if err != nil {
			return r, err
		}

		bs, ds, err := populateBrickSet(
			opts,
			dsrc.Device,
			pred,
			deviceCh,
			brickId)
		if err != nil {
			return r, err
		}
		r.BrickSets = append(r.BrickSets, bs)
		r.DeviceSets = append(r.DeviceSets, ds)
	}

	return r, nil
}

func (bp *StandardBrickPlacer) Replace(
	dsrc DeviceSource,
	opts PlacementOpts,
	pred DeviceFilter,
	bs *BrickSet,
	index int) (
	*BrickAllocation, error) {

	if index < 0 || index >= bs.SetSize {
		return nil, fmt.Errorf(
			"brick replace index out of bounds (got %v, set size %v)",
			index, bs.SetSize)
	}
	logger.Info("Replace brick in brick set %v with index %v",
		bs, index)

	// we return a brick allocation for symmetry with PlaceAll
	// but it only contains one pair of sets
	r := &BrickAllocation{
		BrickSets:  []*BrickSet{NewBrickSet(bs.SetSize)},
		DeviceSets: []*DeviceSet{NewDeviceSet(bs.SetSize)},
	}

	brickId := idgen.GenUUID()
	a := NewSimpleAllocator()
	deviceCh, done, err := a.GetNodesFromDeviceSource(dsrc, brickId)
	defer close(done)
	if err != nil {
		return r, err
	}

	newBrickEntry, newDeviceEntry, err := findDeviceAndBrickForSet(
		opts, dsrc.Device, pred, deviceCh, bs.Drop(index))
	if err != nil {
		return r, err
	}
	newBrickEntry.SetId(brickId)

	// if this all seems like an awful lot of boilerplate
	// and busy work, consider that in real gluster the positions
	// of the bricks w/in the brickset are meaningful and
	// this will make more sense in future position-aware placers
	// (e.g. arbiter)
	newBricks := make([]*BrickEntry, bs.SetSize)
	newDevices := make([]*DeviceEntry, bs.SetSize)
	for i := 0; i < bs.SetSize; i++ {
		if i == index {
			newBricks[i] = newBrickEntry
			newDevices[i] = newDeviceEntry
		} else {
			newBricks[i] = bs.Bricks[i]
			d, err := dsrc.Device(bs.Bricks[i].Info.DeviceId)
			if err != nil {
				return r, err
			}
			newDevices[i] = d
		}
	}
	r.BrickSets[0].Bricks = newBricks
	r.DeviceSets[0].Devices = newDevices

	godbc.Require(r.BrickSets[0].Full())
	godbc.Require(r.DeviceSets[0].Full())
	return r, nil
}
