//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

type DeviceAndNode struct {
	Device *DeviceEntry
	Node   *NodeEntry
}

// DeviceSource is an abstraction used by the BrickPlacer to
// get an initial list of devices where bricks can be placed
// as well as converting device IDs to device entry objects.
// The idea is to keep db/connection/caching logic away from
// the placement algorithms in the placer interface.
type DeviceSource interface {
	// Devices returns a list of all the Device entries and the
	// nodes that the device is on that can
	// be considered for the upcoming brick placement.
	Devices() ([]DeviceAndNode, error)
	// Device looks up a device id and resolves it to a Device
	// entry object.
	Device(id string) (*DeviceEntry, error)
	// Node looks up a node id and resolves it to a Node entry
	// object.
	Node(id string) (*NodeEntry, error)
}

// PlacementOpts is an interface that is meant for passing the
// somewhat complex set of options needed by the placer code
// and hiding the sources of the these values away from the
// placer implementations.
type PlacementOpts interface {
	// BrickSizes returns values needed to calculate the
	// size of the brick on disk.
	BrickSizes() (uint64, float64)
	// BrickOwner returns the ID of object that will "own" the brick
	BrickOwner() string
	// BrickGid return the ID of the GID the brick will use
	BrickGid() int64
	// SetSize returns the size of the Brick Sets that will be
	// allocated.
	SetSize() int
	// SetCount returns the total number of Brick Sets that
	// will be produced.
	SetCount() int
	// AverageFileSize returns the average file size for the volume
	AverageFileSize() uint64
}

// DeviceFilter functions can be defined by the caller of a
// BrickPlacer to define what devices it wants the Placer
// algorithm to exclude from the brick set.
type DeviceFilter func(*BrickSet, *DeviceEntry) bool

// BrickPlacer implementations take their source devices and
// options and place new bricks on devices (if possible).
// The exact placement depends on the implementation and
// the input options.
type BrickPlacer interface {
	// PlaceAll constructs a full sequence of brick sets and
	// corresponding device sets for those bricks.
	PlaceAll(DeviceSource, PlacementOpts, DeviceFilter) (
		*BrickAllocation, error)

	// Replace constructs a brick allocation constrained to
	// a single brick set where the brick set is already populated
	// but a brick with the given index into the set needs to
	// be replaced.
	Replace(DeviceSource, PlacementOpts, DeviceFilter, *BrickSet, int) (
		*BrickAllocation, error)
}

type BrickSubType int

const (
	UnknownSubType BrickSubType = iota
	NormalSubType
	ArbiterSubType
)
