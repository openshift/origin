// +build dbexamples

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
	"testing"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/tests"
)

func buildCluster(app *App) {
	app.db.Update(func(tx *bolt.Tx) error {
		// create a cluster
		cluster_req := &api.ClusterCreateRequest{
			ClusterFlags: api.ClusterFlags{
				Block: true,
				File:  true,
			},
		}
		c := NewClusterEntryFromRequest(cluster_req)
		// create three nodes
		for i := 0; i < 3; i++ {
			node_req := &api.NodeAddRequest{
				ClusterId: "asdf",
				Zone:      1,
				Hostnames: api.HostAddresses{
					Manage:  []string{fmt.Sprintf("mng%v", i)},
					Storage: []string{fmt.Sprintf("stor%v", i)},
				},
			}
			n := NewNodeEntryFromRequest(node_req)
			n.Info.ClusterId = c.Info.Id
			c.NodeAdd(n.Info.Id)

			// create three 1TB devices
			for j := 0; j < 3; j++ {
				dev_req := &api.DeviceAddRequest{
					NodeId: n.Info.Id,
				}
				dev_req.Name = fmt.Sprintf("/dev/id%v", j)
				d := NewDeviceEntryFromRequest(dev_req)
				d.StorageSet(1<<30, 1<<30, 0)
				n.DeviceAdd(d.Id())
				if err := d.Save(tx); err != nil {
					return err
				}
			}
			if err := n.Save(tx); err != nil {
				return err
			}
		}
		if err := c.Save(tx); err != nil {
			return err
		}
		return nil
	})
}

func BuildSimpleCluster(t *testing.T, filename string) {
	app := NewTestApp(filename)
	defer app.Close()

	buildCluster(app)
}

func LeakPendingVolumeCreate(t *testing.T, filename string) {

	app := NewTestApp(filename)
	defer app.Close()

	buildCluster(app)

	req := &api.VolumeCreateRequest{}
	req.Size = 10
	req.Durability.Type = api.DurabilityReplicate
	req.Durability.Replicate.Replica = 3

	vol := NewVolumeEntryFromRequest(req)
	vc := NewVolumeCreateOperation(vol, app.db)

	// verify that there are no volumes, bricks or pending operations
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 0, "expected len(vl) == 0, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 0, "expected len(bl) == 0, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 0, "expected len(pol) == 0, got", len(pol))
		return nil
	})

	e := vc.Build()
	tests.Assert(t, e == nil, "expected e == nil, got", e)

	// verify volumes, bricks, & pending ops exist
	app.db.View(func(tx *bolt.Tx) error {
		vl, e := VolumeList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(vl) == 1, "expected len(vl) == 1, got", len(vl))
		bl, e := BrickList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(bl) == 3, "expected len(bl) == 3, got", len(bl))
		pol, e := PendingOperationList(tx)
		tests.Assert(t, e == nil, "expected e == nil, got", e)
		tests.Assert(t, len(pol) == 1, "expected len(pol) == 1, got", len(pol))

		for _, vid := range vl {
			v, e := NewVolumeEntryFromId(tx, vid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, v.Pending.Id == pol[0],
				"expected v.Pending.Id == pol[0], got:", v.Pending.Id, pol[0])
		}
		for _, bid := range bl {
			b, e := NewBrickEntryFromId(tx, bid)
			tests.Assert(t, e == nil, "expected e == nil, got", e)
			tests.Assert(t, b.Pending.Id == pol[0],
				"expected b.Pending.Id == pol[0], got:", b.Pending.Id, pol[0])
		}
		return nil
	})
}
