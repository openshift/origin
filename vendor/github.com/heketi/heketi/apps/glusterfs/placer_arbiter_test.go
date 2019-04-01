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
	"strings"
	"testing"

	"github.com/heketi/tests"
)

type TestDeviceSource struct {
	devices      map[string]*DeviceEntry
	nodes        map[string]*NodeEntry
	devicesError error
}

func NewTestDeviceSource() *TestDeviceSource {
	return &TestDeviceSource{
		devices: map[string]*DeviceEntry{},
		nodes:   map[string]*NodeEntry{},
	}
}

func (tds *TestDeviceSource) AddDevice(d *DeviceEntry) {
	tds.devices[d.Info.Id] = d
}

func (tds *TestDeviceSource) AddNode(n *NodeEntry) {
	tds.nodes[n.Info.Id] = n
}

func (tds *TestDeviceSource) QuickAdd(
	nodeId, deviceId, dname string, size uint64) {

	tds.MultiAdd(nodeId)(deviceId, dname, size)
}

func (tds *TestDeviceSource) MultiAdd(nodeId string) func(string, string, uint64) {

	n := NewNodeEntry()
	n.Info.Id = nodeId
	n.Info.Zone = 1
	n.Info.Hostnames.Manage = []string{"mng-" + nodeId}
	n.Info.Hostnames.Storage = []string{"stor-" + nodeId}
	n.Info.ClusterId = "0000000000c"
	tds.AddNode(n)

	return func(deviceId, dname string, size uint64) {
		d := NewDeviceEntry()
		d.Info.Id = deviceId
		d.Info.Name = dname
		d.Info.Storage.Total = size
		d.Info.Storage.Free = size
		d.NodeId = nodeId

		n.Devices = append(n.Devices, d.Info.Id)
		tds.AddDevice(d)
	}
}

func (tds *TestDeviceSource) Devices() ([]DeviceAndNode, error) {
	if tds.devicesError != nil {
		return nil, tds.devicesError
	}
	valid := [](DeviceAndNode){}
	for _, node := range tds.nodes {
		for _, deviceId := range node.Devices {
			device, ok := tds.devices[deviceId]
			if !ok {
				return nil, ErrNotFound
			}
			valid = append(valid, DeviceAndNode{
				Device: device,
				Node:   node,
			})
		}
	}
	return valid, nil
}

func (tds *TestDeviceSource) Device(id string) (*DeviceEntry, error) {
	if device, ok := tds.devices[id]; ok {
		return device, nil
	}
	return nil, ErrNotFound
}

func (tds *TestDeviceSource) Node(id string) (*NodeEntry, error) {
	if node, ok := tds.nodes[id]; ok {
		return node, nil
	}
	return nil, ErrNotFound
}

type TestPlacementOpts struct {
	brickSize       uint64
	brickSnapFactor float64
	brickOwner      string
	brickGid        int64
	setSize         int
	setCount        int
	averageFileSize uint64
}

func (tpo *TestPlacementOpts) BrickSizes() (uint64, float64) {
	return tpo.brickSize, tpo.brickSnapFactor
}

func (tpo *TestPlacementOpts) BrickOwner() string {
	return tpo.brickOwner
}

func (tpo *TestPlacementOpts) BrickGid() int64 {
	return tpo.brickGid
}

func (tpo *TestPlacementOpts) SetSize() int {
	return tpo.setSize
}

func (tpo *TestPlacementOpts) SetCount() int {
	return tpo.setCount
}

func (tpo *TestPlacementOpts) AverageFileSize() uint64 {
	return tpo.averageFileSize
}

func TestTestDeviceSource(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		1100)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		1200)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		1300)

	tests.Assert(t, len(dsrc.devices) == 3,
		"expected len(dsrc.devices) == 3, got:", len(dsrc.devices))
	tests.Assert(t, len(dsrc.nodes) == 3,
		"expected len(dsrc.nodes) == 3, got:", len(dsrc.nodes))

	d, err := dsrc.Device("22222222")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, d.Info.Storage.Total == 1200,
		"expected d.Info.Storage.Total == 1200, got:", d.Info.Storage.Total)

	d, err = dsrc.Device("10000000")
	tests.Assert(t, err == ErrNotFound, "expected err == ErrNotFound, got:", err)

	n, err := dsrc.Node("30000000")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, n.Info.Hostnames.Manage[0] == "mng-30000000",
		"expected n.Info.Hostnames.Manage[0] == \"mng-30000000\", got:",
		n.Info.Hostnames.Manage[0])

	n, err = dsrc.Node("abcdefgh")
	tests.Assert(t, err == ErrNotFound, "expected err == ErrNotFound, got:", err)

	dnl, err := dsrc.Devices()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(dnl) == 3,
		"expected len(dnl) == 3, got:", len(dnl))
}

func TestTestDeviceSourceMultiAdd(t *testing.T) {
	dsrc := NewTestDeviceSource()
	addDev := dsrc.MultiAdd("abcd")
	addDev("d1", "/dev/x1", 10)
	addDev("d2", "/dev/x2", 20)
	addDev("d3", "/dev/x3", 30)
	addDev = dsrc.MultiAdd("foo")
	addDev("d4", "/dev/x1", 40)
	addDev("d5", "/dev/x2", 50)
	dsrc.MultiAdd("bar")("d6", "/dev/x1", 60)

	tests.Assert(t, len(dsrc.devices) == 6,
		"expected len(dsrc.devices) == 6, got:", len(dsrc.devices))
	tests.Assert(t, len(dsrc.nodes) == 3,
		"expected len(dsrc.nodes) == 3, got:", len(dsrc.nodes))

	dnl, err := dsrc.Devices()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(dnl) == 6,
		"expected len(dnl) == 6, got:", len(dnl))
}

func TestArbiterBrickPlacer(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		100*GB)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		100*GB)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		100*GB)
	opts := &TestPlacementOpts{
		brickSize:       10 * GB,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))

	tests.Assert(t, ba.BrickSets[0].Full(),
		"expected ba.BrickSets[0].Full() to be true, was false")
	tests.Assert(t, ba.BrickSets[0].SetSize == 3,
		"expected ba.BrickSets[0].SetSize == 3, got:",
		ba.BrickSets[0].SetSize)
	bs := ba.BrickSets[0]
	tests.Assert(t, bs.Bricks[0].Info.Size == bs.Bricks[1].Info.Size,
		"expected bs.Bricks[0].Info.Size == bs.Bricks[1].Info.Size, got:",
		bs.Bricks[0].Info.Size, bs.Bricks[1].Info.Size)
	tests.Assert(t, bs.Bricks[0].Info.Size > bs.Bricks[2].Info.Size,
		"expected bs.Bricks[0].Info.Size > bs.Bricks[2].Info.Size, got:",
		bs.Bricks[0].Info.Size, bs.Bricks[2].Info.Size)
}

func TestArbiterBrickPlacerTooSmall(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		810)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		820)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		830)
	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	_, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == ErrNoSpace, "expected err == ErrNoSpace, got:", err)
}

func TestArbiterBrickPlacerDevicesFail(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.devicesError = fmt.Errorf("Zonk!")

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	_, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == dsrc.devicesError,
		"expected err == dsrc.devicesError, got:", err)
}

func TestArbiterBrickPlacerPredicateBlock(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		11000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		12000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		13000)
	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	pred := func(bs *BrickSet, d *DeviceEntry) bool {
		return false
	}
	_, err := abplacer.PlaceAll(dsrc, opts, pred)
	tests.Assert(t, err == ErrNoSpace, "expected err == ErrNoSpace, got:", err)
}

func TestArbiterBrickPlacerBrickOnArbiterDevice(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"a3333333",
		"/dev/foobar",
		23000)
	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	abplacer.canHostArbiter = func(d *DeviceEntry, ds DeviceSource) bool {
		return d.Info.Id[0] == 'a'
	}
	abplacer.canHostData = func(d *DeviceEntry, ds DeviceSource) bool {
		return !abplacer.canHostArbiter(d, ds)
	}
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, ba.BrickSets[0].Bricks[2].Info.DeviceId == "a3333333",
		`expected ba.BrickSets[0].Bricks[2].Info.DeviceId == "a3333333", got:`,
		ba.BrickSets[0].Bricks[2].Info.DeviceId)
	tests.Assert(t, ba.DeviceSets[0].Devices[2].Info.Id == "a3333333",
		`expected ba.DeviceSets[0].Devices[2].Info.Id == "a3333333", got`,
		ba.DeviceSets[0].Devices[2].Info.Id)
}

func TestArbiterBrickPlacerBrickThreeSets(t *testing.T) {
	dsrc := NewTestDeviceSource()
	addDev := dsrc.MultiAdd("10000000")
	addDev("11111111", "/dev/d1", 20001)
	addDev("21111111", "/dev/d2", 20002)
	addDev("31111111", "/dev/d3", 20003)
	addDev = dsrc.MultiAdd("20000000")
	addDev("41111111", "/dev/d1", 20001)
	addDev("51111111", "/dev/d2", 20002)
	addDev("61111111", "/dev/d3", 20003)
	addDev = dsrc.MultiAdd("30000000")
	addDev("71111111", "/dev/d1", 20001)
	addDev("81111111", "/dev/d2", 20002)
	addDev("91111111", "/dev/d3", 20003)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        3,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 3,
		"expected len(ba.BrickSets) == 3, got:", len(ba.BrickSets))

	assertNoRepeatNodesInBrickSet(t, dsrc, ba)
}

func TestArbiterBrickPlacerBrickThreeSetsOnArbiterDevice(t *testing.T) {
	dsrc := NewTestDeviceSource()
	addDev := dsrc.MultiAdd("10000000")
	// data nodes
	addDev("11111111", "/dev/d1", 100*GB)
	addDev("21111111", "/dev/d2", 100*GB)
	addDev = dsrc.MultiAdd("20000000")
	addDev("31111111", "/dev/d1", 100*GB)
	addDev("41111111", "/dev/d2", 100*GB)
	addDev = dsrc.MultiAdd("30000000")
	addDev("51111111", "/dev/d1", 100*GB)
	addDev("61111111", "/dev/d2", 100*GB)
	addDev = dsrc.MultiAdd("40000000")
	addDev("71111111", "/dev/d1", 100*GB)
	addDev("81111111", "/dev/d2", 100*GB)
	// arbiter nodes
	addDev = dsrc.MultiAdd("50000000")
	addDev("a1111111", "/dev/d1", 100*GB)
	addDev = dsrc.MultiAdd("60000000")
	addDev("a2111111", "/dev/d1", 100*GB)
	addDev = dsrc.MultiAdd("70000000")
	addDev("a3111111", "/dev/d1", 100*GB)
	// the above configuration is pretty artificial and reflects
	// a downside to the current approach. because of the
	// non-deterministic way the ring provides devices and
	// the (hard) requirement not to reuse a node within the brick
	// set its fairly easy with small devices to run into a situation
	// where the placement fails even though the configuration could
	// have hosted the volume. There are things we can do to
	// improve this but not now and not for this test. :-\

	opts := &TestPlacementOpts{
		brickSize:       10 * GB,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        3,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	abplacer.canHostArbiter = func(d *DeviceEntry, ds DeviceSource) bool {
		return d.Info.Id[0] == 'a'
	}
	abplacer.canHostData = func(d *DeviceEntry, ds DeviceSource) bool {
		return !abplacer.canHostArbiter(d, ds)
	}
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 3,
		"expected len(ba.BrickSets) == 3, got:", len(ba.BrickSets))
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			brickDeviceId := ba.BrickSets[i].Bricks[j].Info.DeviceId
			prefixA := (brickDeviceId[0] == 'a')
			if j == 2 {
				tests.Assert(t, prefixA, "expected prefixA true on index", j)
			} else {
				tests.Assert(t, !prefixA, "expected prefixA false on index", j)
			}
		}
	}
	assertNoRepeatNodesInBrickSet(t, dsrc, ba)
}

func TestArbiterBrickPlacerSimpleReplace(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		23000)
	dsrc.QuickAdd(
		"40000000",
		"44444444",
		"/dev/foobar",
		23000)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
		"expected len(ba.BrickSets[0].Bricks) == 3, got:",
		len(ba.BrickSets[0].Bricks))

	ba2, err := abplacer.Replace(dsrc, opts, nil, ba.BrickSets[0], 0)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba2.BrickSets) == 1,
		"expected len(ba2.BrickSets) == 1, got:", len(ba2.BrickSets))
	tests.Assert(t, len(ba2.BrickSets[0].Bricks) == 3,
		"expected len(ba2.BrickSets[0].Bricks) == 3, got:",
		len(ba2.BrickSets[0].Bricks))

	assertNoRepeatNodesInBrickSet(t, dsrc, ba)
	assertNoRepeatNodesInBrickSet(t, dsrc, ba2)

	bs1 := ba.BrickSets[0]
	bs2 := ba2.BrickSets[0]
	// we replaced the 1st brick, thus it should differ
	tests.Assert(t,
		bs1.Bricks[0].Info.Id != bs2.Bricks[0].Info.Id,
		"expected bs1.Bricks[0].Info.Id == bs2.Bricks[0].Info.Id, got:",
		bs1.Bricks[0].Info.Id, bs2.Bricks[0].Info.Id)
	// the remaining bricks will be the same
	tests.Assert(t,
		bs1.Bricks[1].Info.Id == bs2.Bricks[1].Info.Id,
		"expected bs1.Bricks[1].Info.Id == bs2.Bricks[1].Info.Id, got:",
		bs1.Bricks[1].Info.Id, bs2.Bricks[1].Info.Id)
	tests.Assert(t,
		bs1.Bricks[2].Info.Id == bs2.Bricks[2].Info.Id,
		"expected bs1.Bricks[2].Info.Id == bs2.Bricks[2].Info.Id, got:",
		bs1.Bricks[2].Info.Id, bs2.Bricks[2].Info.Id)
}

func TestArbiterBrickPlacerReplaceIndexOOB(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		23000)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
		"expected len(ba.BrickSets[0].Bricks) == 3, got:",
		len(ba.BrickSets[0].Bricks))

	_, err = abplacer.Replace(dsrc, opts, nil, ba.BrickSets[0], -1)
	tests.Assert(t, strings.Contains(err.Error(), "out of bounds"),
		`expected strings.Contains(err.Error(), "out of bounds"), got:`,
		err.Error())

	_, err = abplacer.Replace(dsrc, opts, nil, ba.BrickSets[0], 9)
	tests.Assert(t, strings.Contains(err.Error(), "out of bounds"),
		`expected strings.Contains(err.Error(), "out of bounds"), got:`,
		err.Error())
}

func TestArbiterBrickPlacerReplaceDevicesFail(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		23000)
	dsrc.QuickAdd(
		"40000000",
		"44444444",
		"/dev/foobar",
		23000)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
		"expected len(ba.BrickSets[0].Bricks) == 3, got:",
		len(ba.BrickSets[0].Bricks))

	dsrc.devicesError = fmt.Errorf("Zonk!")
	_, err = abplacer.Replace(dsrc, opts, nil, ba.BrickSets[0], 0)
	tests.Assert(t, err == dsrc.devicesError,
		"expected err == dsrc.devicesError, got:", err)
}

func TestArbiterBrickPlacerReplaceTooFew(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		23000)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
		"expected len(ba.BrickSets[0].Bricks) == 3, got:",
		len(ba.BrickSets[0].Bricks))

	pred := func(bs *BrickSet, d *DeviceEntry) bool {
		return ba.BrickSets[0].Bricks[0].Info.DeviceId != d.Info.Id
	}
	_, err = abplacer.Replace(dsrc, opts, pred, ba.BrickSets[0], 0)
	tests.Assert(t, err == ErrNoSpace,
		"expected err == ErrNoSpace, got:", err)
}

func TestArbiterBrickPlacerReplaceTooFewArbiter(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"10000000",
		"11111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"20000000",
		"22222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"30000000",
		"33333333",
		"/dev/foobar",
		23000)
	dsrc.QuickAdd(
		"40000000",
		"44444444",
		"/dev/foobar",
		24000)
	dsrc.QuickAdd(
		"50000000",
		"a5555555",
		"/dev/foobar",
		25000)
	// we have enough devices for a generic replace but not
	// when we're limited to certain devices

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	abplacer.canHostArbiter = func(d *DeviceEntry, ds DeviceSource) bool {
		return d.Info.Id[0] == 'a'
	}
	abplacer.canHostData = func(d *DeviceEntry, ds DeviceSource) bool {
		return !abplacer.canHostArbiter(d, ds)
	}

	ba, err := abplacer.PlaceAll(dsrc, opts, nil)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba.BrickSets) == 1,
		"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
	tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
		"expected len(ba.BrickSets[0].Bricks) == 3, got:",
		len(ba.BrickSets[0].Bricks))

	// this will fail because we have no more "arbiter devices"
	pred := func(bs *BrickSet, d *DeviceEntry) bool {
		return ba.BrickSets[0].Bricks[2].Info.DeviceId != d.Info.Id
	}
	_, err = abplacer.Replace(dsrc, opts, pred, ba.BrickSets[0], 2)
	tests.Assert(t, err == ErrNoSpace,
		"expected err == ErrNoSpace, got:", err)

	// this one will work because the free device is not arbiter
	// and the 1 position is a data brick
	ba2, err := abplacer.Replace(dsrc, opts, nil, ba.BrickSets[0], 1)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(ba2.BrickSets) == 1,
		"expected len(ba2.BrickSets) == 1, got:", len(ba2.BrickSets))
	tests.Assert(t, len(ba2.BrickSets[0].Bricks) == 3,
		"expected len(ba2.BrickSets[0].Bricks) == 3, got:",
		len(ba2.BrickSets[0].Bricks))
}

func TestArbiterBrickPlacerArbiterBrickPreference(t *testing.T) {
	dsrc := NewTestDeviceSource()
	dsrc.QuickAdd(
		"a1000000",
		"a1111111",
		"/dev/foobar",
		21000)
	dsrc.QuickAdd(
		"a2000000",
		"a2222222",
		"/dev/foobar",
		22000)
	dsrc.QuickAdd(
		"a3000000",
		"a3333333",
		"/dev/foobar",
		23000)
	dsrc.QuickAdd(
		"40000000",
		"44444444",
		"/dev/foobar",
		24000)
	dsrc.QuickAdd(
		"50000000",
		"55555555",
		"/dev/foobar",
		25000)
	dsrc.QuickAdd(
		"60000000",
		"66666666",
		"/dev/foobar",
		26000)

	opts := &TestPlacementOpts{
		brickSize:       800,
		brickSnapFactor: 0.3,
		brickOwner:      "asdfasdf",
		setSize:         3,
		setCount:        1,
		averageFileSize: 64 * KB,
	}

	abplacer := NewArbiterBrickPlacer()
	abplacer.canHostArbiter = func(d *DeviceEntry, ds DeviceSource) bool {
		// any device *can* host arbiter
		return true
	}
	abplacer.canHostData = func(d *DeviceEntry, ds DeviceSource) bool {
		// data bricks may not land on "a" devices
		return d.Info.Id[0] != 'a'
	}

	for i := 0; i < 3; i++ {
		ba, err := abplacer.PlaceAll(dsrc, opts, nil)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(ba.BrickSets) == 1,
			"expected len(ba.BrickSets) == 1, got:", len(ba.BrickSets))
		tests.Assert(t, len(ba.BrickSets[0].Bricks) == 3,
			"expected len(ba.BrickSets[0].Bricks) == 3, got:",
			len(ba.BrickSets[0].Bricks))

		bs := ba.BrickSets[0]
		tests.Assert(t, bs.Bricks[2].Info.DeviceId[0] == 'a',
			"expected bs.Bricks[2].Info.DeviceId[0] == 'a', got:",
			bs.Bricks[2].Info.DeviceId)
	}
}

func assertNoRepeatNodesInBrickSet(t *testing.T,
	dsrc DeviceSource, ba *BrickAllocation) {

	// this check is only for arbiter tests so we can assume that there
	// will be exactly 3 bricks in a brickset on successful placement
	for _, bs := range ba.BrickSets {
		d0, err := dsrc.Device(bs.Bricks[0].Info.DeviceId)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		d1, err := dsrc.Device(bs.Bricks[1].Info.DeviceId)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		d2, err := dsrc.Device(bs.Bricks[2].Info.DeviceId)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, d0.NodeId != d1.NodeId,
			"bricks 0 1 placed on same node:", d0.NodeId)
		tests.Assert(t, d1.NodeId != d2.NodeId,
			"bricks 1 2 placed on same node:", d1.NodeId)
		tests.Assert(t, d2.NodeId != d0.NodeId,
			"bricks 2 0 placed on same node:", d2.NodeId)
	}
}
