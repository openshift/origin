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

	"github.com/heketi/heketi/pkg/idgen"
)

var (
	tryPlaceAgain error = fmt.Errorf("Placement failed. Try again.")
)

const (
	arbiter_index int = 2
)

// ArbiterBrickPlacer is a Brick Placer implementation that can
// place bricks for arbiter volumes. It works primarily by
// dividing the devices into two "pools" - one for data bricks
// and one for arbiter bricks and understanding that only
// the last brick in the brick set is an arbiter brick.
type ArbiterBrickPlacer struct {
	// the following two function vars are to better support
	// dep. injection & unit testing
	canHostArbiter func(*DeviceEntry, DeviceSource) bool
	canHostData    func(*DeviceEntry, DeviceSource) bool
}

// Arbiter opts supports passing arbiter specific options
// across layers in the arbiter code along with the
// original placement opts.
type arbiterOpts struct {
	o         PlacementOpts
	brickSize uint64
	// used to determine if the device should be
	// updated with the brick ID
	recordBrick bool
}

func newArbiterOpts(opts PlacementOpts) *arbiterOpts {
	bsize, _ := opts.BrickSizes()
	return &arbiterOpts{
		o:         opts,
		brickSize: bsize,
		// by default we want to record bricks
		// this needs to be set to false in the replace path
		recordBrick: true,
	}
}

func (aopts *arbiterOpts) discount(index int) (err error) {
	if index == arbiter_index {
		aopts.brickSize, err = discountBrickSize(
			aopts.brickSize, aopts.o.AverageFileSize())
	}
	return
}

// NewArbiterBrickPlacer returns a new placer for bricks in
// a volume that supports the arbiter feature.
func NewArbiterBrickPlacer() *ArbiterBrickPlacer {
	return &ArbiterBrickPlacer{
		canHostArbiter: func(d *DeviceEntry, dsrc DeviceSource) bool {
			return deviceHasArbiterTag(d, dsrc,
				TAG_VAL_ARBITER_REQUIRED, TAG_VAL_ARBITER_SUPPORTED)
		},
		canHostData: func(d *DeviceEntry, dsrc DeviceSource) bool {
			return deviceHasArbiterTag(d, dsrc,
				TAG_VAL_ARBITER_SUPPORTED, TAG_VAL_ARBITER_DISABLED)
		},
	}
}

// PlaceAll constructs a full BrickAllocation for a volume that
// supports the arbiter feature.
func (bp *ArbiterBrickPlacer) PlaceAll(
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
		bs, ds, err := bp.newSets(
			dsrc,
			opts,
			pred)
		if err != nil {
			return r, err
		}
		if bs.IsSparse() {
			return r, fmt.Errorf("Did not fully populate brick set")
		}
		if ds.IsSparse() {
			return r, fmt.Errorf("Did not fully populate device set")
		}
		r.BrickSets = append(r.BrickSets, bs)
		r.DeviceSets = append(r.DeviceSets, ds)
	}

	return r, nil
}

// Replace swaps out a brick & device in the input brick set at the
// given index for a new brick.
func (bp *ArbiterBrickPlacer) Replace(
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
	wbs := r.BrickSets[0]
	wds := r.DeviceSets[0]

	dscan, err := bp.Scanner(dsrc)
	if err != nil {
		return r, err
	}
	defer dscan.Close()

	// copy input brick set to working brick set and get the
	// corresponding device entries for the device set
	for i, b := range bs.Bricks {
		d, err := dsrc.Device(b.Info.DeviceId)
		if err != nil {
			return r, err
		}
		wbs.Insert(i, b)
		wds.Insert(i, d)
	}
	aopts := newArbiterOpts(opts)
	// this is a mildly hacky way to deal with the higher level replace
	// code's desire to save the device size and the device's bricks
	// in different db commits. Eventually we should move to a more
	// unified approach and drop this
	aopts.recordBrick = false
	err = bp.placeBrickInSet(dsrc, dscan, aopts, pred, wbs, wds, index)
	return r, err
}

// newSets returns a new fully populated pair of brick and device sets.
// If new sets can not be placed err will be non-nil.
func (bp *ArbiterBrickPlacer) newSets(
	dsrc DeviceSource,
	opts PlacementOpts,
	pred DeviceFilter) (*BrickSet, *DeviceSet, error) {

	ssize := opts.SetSize()
	bs := NewSparseBrickSet(ssize)
	ds := NewSparseDeviceSet(ssize)
	dscan, err := bp.Scanner(dsrc)
	if err != nil {
		return nil, nil, err
	}
	defer dscan.Close()

	// work backwards from the last item in the brick set (typically index 2)
	// in order to place the most special brick, the arbiter brick, first.
	// Placing the arbiter brick first means we get a more reliable distribution
	// of bricks because the nodes will not be taken by the two data
	// bricks before we try to place the arbiter brick.
	for index := ssize - 1; index >= 0; index-- {
		aopts := newArbiterOpts(opts)
		if e := aopts.discount(index); e != nil {
			return bs, ds, e
		}
		err := bp.placeBrickInSet(dsrc, dscan, aopts, pred, bs, ds, index)
		if err != nil {
			return bs, ds, err
		}
	}
	return bs, ds, nil
}

// placeBrickInSet uses the device scanner to find a device suitable
// for a new brick at the given index. If no devices can be found for
// the brick err will be non-nil.
func (bp *ArbiterBrickPlacer) placeBrickInSet(
	dsrc DeviceSource,
	dscan *arbiterDeviceScanner,
	opts *arbiterOpts,
	pred DeviceFilter,
	bs *BrickSet,
	ds *DeviceSet,
	index int) error {

	logger.Info("Placing brick in brick set at position %v", index)
	for deviceId := range dscan.Scan(index) {

		device, err := dsrc.Device(deviceId)
		if err != nil {
			return err
		}

		err = bp.tryPlaceBrickOnDevice(
			opts, pred, bs, ds, index, device)
		switch err {
		case tryPlaceAgain:
			continue
		case nil:
			logger.Debug("Placed brick at index %v on device %v",
				index, deviceId)
			return nil
		default:
			return err
		}
	}

	// we exhausted all possible devices for this brick
	logger.Debug("Can not find any device for brick (index=%v)", index)
	return ErrNoSpace
}

// tryPlaceBrickOnDevice attempts to place a brick on the given device.
// If placement is successful the brick and device sets are updated,
// and the error is nil.
// If placement fails then tryPlaceAgain error is returned.
func (bp *ArbiterBrickPlacer) tryPlaceBrickOnDevice(
	opts *arbiterOpts,
	pred DeviceFilter,
	bs *BrickSet,
	ds *DeviceSet,
	index int,
	device *DeviceEntry) error {

	logger.Debug("Trying to place brick on device %v (node %v)",
		device.Info.Id, device.NodeId)

	for i, b := range bs.Bricks {
		// do not check the brick in the brick set for the current
		// index. If this is a new brick set we won't have the index
		// populated. If this is a replace, we will have the old brick
		// at the index and we are OK with re-using its node (as the
		// standard placer does)
		// If b is nil, it means that this is a "sparse" brick set and
		// we have not tried allocating a brick for that index yet,
		// so there's nothing to check.
		if i == index || b == nil {
			continue
		}
		if b.Info.NodeId == device.NodeId {
			// this node is used by an existing brick in the set
			// we can not use this device
			logger.Debug("Node %v already in use by brick set (device %v)",
				device.NodeId, device.Info.Id)
			return tryPlaceAgain
		}
	}

	if pred != nil && !pred(bs, device) {
		logger.Debug("Device %v rejected by predicate function", device.Info.Id)
		return tryPlaceAgain
	}

	// Try to allocate a brick on this device
	origBrickSize, snapFactor := opts.o.BrickSizes()
	brickSize := opts.brickSize
	if brickSize != origBrickSize {
		logger.Info("Placing brick with discounted size: %v", brickSize)
	}
	brick := device.NewBrickEntry(brickSize, snapFactor,
		opts.o.BrickGid(), opts.o.BrickOwner())
	if brick == nil {
		logger.Debug(
			"Unable to place a brick of size %v & factor %v on device %v",
			brickSize, snapFactor, device.Info.Id)
		return tryPlaceAgain
	}
	if index == arbiter_index {
		brick.SubType = ArbiterSubType
	} else {
		brick.SubType = NormalSubType
	}

	if opts.recordBrick {
		device.BrickAdd(brick.Id())
	}
	bs.Insert(index, brick)
	ds.Insert(index, device)
	return nil
}

type deviceFeed struct {
	Devices <-chan string
	Done    chan struct{}
}

type arbiterDeviceScanner struct {
	arbiter deviceFeed
	data    deviceFeed
}

func deviceFeedFromRings(id string, ring ...*SimpleAllocatorRing) deviceFeed {
	devices := make(chan string)
	done := make(chan struct{})
	d := SimpleDevices{}
	for _, r := range ring {
		d = append(d, r.GetDeviceList(id)...)
	}
	generateDevices(d, devices, done)
	return deviceFeed{
		Devices: devices,
		Done:    done,
	}
}

// Scanner returns a pointer to an arbiterDeviceScanner helper object.
// This object can be used to range over the devices that a brick
// may be placed on. The .Close method must be called to release
// resources associated with this object.
func (bp *ArbiterBrickPlacer) Scanner(dsrc DeviceSource) (
	*arbiterDeviceScanner, error) {

	dataRing := NewSimpleAllocatorRing()
	arbiterRing := NewSimpleAllocatorRing()
	anyRing := NewSimpleAllocatorRing()
	dnl, err := dsrc.Devices()
	if err != nil {
		return nil, err
	}
	for _, dan := range dnl {
		sd := &SimpleDevice{
			zone:     dan.Node.Info.Zone,
			nodeId:   dan.Node.Info.Id,
			deviceId: dan.Device.Info.Id,
		}
		// it is perfectly fine for a device to host data & arbiter
		// bricks if it is so configured.
		arbiterOk := bp.canHostArbiter(dan.Device, dsrc)
		dataOk := bp.canHostData(dan.Device, dsrc)
		switch {
		case arbiterOk && dataOk:
			anyRing.Add(sd)
		case arbiterOk:
			arbiterRing.Add(sd)
		case dataOk:
			dataRing.Add(sd)
		default:
			logger.Warning("device %v does not support arbiter or data bricks",
				sd.deviceId)
		}
	}

	id := idgen.GenUUID()
	return &arbiterDeviceScanner{
		arbiter: deviceFeedFromRings(id, arbiterRing, anyRing),
		data:    deviceFeedFromRings(id, dataRing, anyRing),
	}, nil
}

// Close releases the resources held by the scanner.
func (dscan *arbiterDeviceScanner) Close() {
	close(dscan.arbiter.Done)
	close(dscan.data.Done)
}

// Scan returns a channel that may be ranged over for eligible devices
// for a brick in a brick set with the position specified by index.
func (dscan *arbiterDeviceScanner) Scan(index int) <-chan string {
	// currently this is hard-coded such that the index of
	// an arbiter brick is always two (the 3rd brick in the set of three)
	// In the future we may want to be smarter here, but this
	// works for now.
	if index == arbiter_index {
		return dscan.arbiter.Devices
	}
	return dscan.data.Devices
}

func discountBrickSize(dataBrickSize, averageFileSize uint64) (brickSize uint64,
	err error) {

	if dataBrickSize < averageFileSize {
		return 0, fmt.Errorf(
			"Average file size (%v) is greater than Brick size (%v)",
			averageFileSize, dataBrickSize)
	}

	brickSize = dataBrickSize / averageFileSize

	if brickSize < 16*MB {
		logger.Info("Increasing calculated arbiter brickSize (%vKiB) "+
			"to 16MiB, the minimum XFS filsystem size with 4KiB "+
			"blocks.", brickSize)
		brickSize = 16 * MB
	}

	return brickSize, nil
}

func deviceHasArbiterTag(d *DeviceEntry, dsrc DeviceSource, v ...string) bool {
	n, err := dsrc.Node(d.NodeId)
	if err != nil {
		logger.LogError("failed to fetch node (%v) for arbiter tag: %v",
			d.NodeId, err)
		return false
	}
	a := ArbiterTag(MergeTags(n, d))
	for _, value := range v {
		if value == a {
			return true
		}
	}
	return false
}
