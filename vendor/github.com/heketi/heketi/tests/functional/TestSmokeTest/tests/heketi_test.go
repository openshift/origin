// +build functional

//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//
package functional

import (
	"fmt"
	"net/http"
	"testing"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/logging"
	"github.com/heketi/heketi/pkg/testutils"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
)

var (
	// Heketi client params
	heketiUrl = "http://localhost:8080"
	heketi    = client.NewClientNoAuth(heketiUrl)

	cenv = &testutils.ClusterEnv{
		HeketiUrl: heketiUrl,
		Nodes: []string{
			"192.168.10.100",
			"192.168.10.101",
			"192.168.10.102",
			"192.168.10.103",
		},
		SSHPort: "22",
		Disks: []string{
			"/dev/vdb",
			"/dev/vdc",
			"/dev/vdd",
			"/dev/vde",
			"/dev/vdf",
			"/dev/vdg",
			"/dev/vdh",
			"/dev/vdi",
		},
	}

	logger = logging.NewLogger("[test]", logging.LEVEL_DEBUG)
)

func setupCluster(t *testing.T, numNodes int, numDisks int) {
	cenv.Update()
	cenv.Setup(t, numNodes, numDisks)
}

func teardownCluster(t *testing.T) {
	cenv.Teardown(t)
}

func TestConnection(t *testing.T) {
	r, err := http.Get(heketiUrl + "/hello")
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, r.StatusCode == http.StatusOK)
}

func TestHeketiSmokeTest(t *testing.T) {

	// Setup the VM storage topology
	teardownCluster(t)
	setupCluster(t, 4, 8)
	defer teardownCluster(t)

	// Create a volume and delete a few time to test garbage collection
	for i := 0; i < 2; i++ {

		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 2500
		volReq.Snapshot.Enable = true
		volReq.Snapshot.Factor = 1.5
		volReq.Durability.Type = api.DurabilityReplicate

		volInfo, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, volInfo.Size == 2500)
		tests.Assert(t, volInfo.Mount.GlusterFS.MountPoint != "")
		tests.Assert(t, volInfo.Name != "")

		volumes, err := heketi.VolumeList()
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
		tests.Assert(t, len(volumes.Volumes) == 1)
		tests.Assert(t, volumes.Volumes[0] == volInfo.Id)

		err = heketi.VolumeDelete(volInfo.Id)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}

	// Create a 1TB volume
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1024
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5
	volReq.Durability.Type = api.DurabilityReplicate

	simplevol, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// Create a 12TB volume with 6TB of snapshot space
	// There should be no space
	volReq = &api.VolumeCreateRequest{}
	volReq.Size = 12 * 1024
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5
	volReq.Durability.Type = api.DurabilityReplicate

	_, err = heketi.VolumeCreate(volReq)
	tests.Assert(t, err != nil)

	// Check there is only one
	volumes, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volumes.Volumes) == 1)

	// Create a 100G volume with replica 3
	volReq = &api.VolumeCreateRequest{}
	volReq.Size = 100
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3

	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, volInfo.Size == 100)
	tests.Assert(t, volInfo.Mount.GlusterFS.MountPoint != "")
	tests.Assert(t, volInfo.Name != "")
	tests.Assert(t, len(volInfo.Bricks) == 3, len(volInfo.Bricks))

	// Check there are two volumes
	volumes, err = heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volumes.Volumes) == 2)

	// Expand volume
	volExpReq := &api.VolumeExpandRequest{}
	volExpReq.Size = 2000

	volInfo, err = heketi.VolumeExpand(simplevol.Id, volExpReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, volInfo.Size == simplevol.Size+2000)

	// Delete volume
	err = heketi.VolumeDelete(volInfo.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestHeketiCreateVolumeWithGid(t *testing.T) {
	na := testutils.RequireNodeAccess(t)
	// Setup the VM storage topology
	teardownCluster(t)
	setupCluster(t, 4, 8)
	defer teardownCluster(t)

	// Create a volume
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1024
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5

	volReq.Gid = 2345

	// Create the volume
	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// SSH into system, create two writers belonging to writegroup gid and
	// make sure both can write and the sticky group bit is set
	exec := na.Use(logger)
	cmd := []string{
		"sudo groupadd -f -g 2345 writegroup",
		"grep -q ^writer1: /etc/passwd || sudo useradd writer1 -G writegroup",
		"grep -q ^writer2: /etc/passwd || sudo useradd writer2 -G writegroup",
		"sudo umount /mnt 2>/dev/null || true",
		fmt.Sprintf("sudo mount -t glusterfs %v /mnt", volInfo.Mount.GlusterFS.MountPoint),
		"sudo runuser -u writer1 -- touch /mnt/writer1testfile",
		"sudo runuser -u writer1 -- mkdir /mnt/writer1dir",
		"sudo runuser -u writer1 -- chmod 770 /mnt/writer1dir",
		"sudo runuser -u writer1 -- touch /mnt/writer1dir/testfile",
		"sudo runuser -u writer2 -- touch /mnt/writer2testfile",
		"sudo runuser -u writer2 -- mkdir /mnt/writer2dir",
		"sudo runuser -u writer2 -- touch /mnt/writer2dir/testfile",
		"sudo runuser -u writer2 -- mkdir /mnt/writer1dir/writer2subdir",
		"sudo runuser -u writer2 -- touch /mnt/writer1dir/writer2testfile",
		"! sudo runuser -u nobody -- touch /mnt/nobodytestfile",
	}
	_, err = exec.ConnectAndExec(cenv.SshHost(0), cmd, 10, false)
	tests.Assert(t, err == nil, err)
}

func TestRemoveDevice(t *testing.T) {

	// Setup the VM storage topology
	teardownCluster(t)
	setupCluster(t, 3, 2)
	defer teardownCluster(t)

	// We have 2 disks of 500GB on every node
	// Total space per node is 1TB
	// We have 3 Nodes, so total space is 3TB

	// vol1: 300 ==> 1 replica set
	// vol2: 600 ==> 4 replica sets of 150 each
	//               on each node:
	//               1 brick on the already used disk
	//               3 bricks on the previously unused disk
	//
	//             n1d1   n2d1   n3d1
	//       -------------------------
	//       r1: [ r1b1 , r1b2, r1b3 ]
	//
	//             n1d2   n2d2   n3d2
	//       -------------------------
	//       r2  [ r2b1,  r2b2,  r2b3 ]
	//       r3  [ r3b1,  r3b2   r3b4 ]
	//       r4  [ r4b1   r4b2   r4b3 ]

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 300
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	vol1, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// Check there is only one
	volumes, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volumes.Volumes) == 1)

	volReq = &api.VolumeCreateRequest{}
	volReq.Size = 600
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	vol2, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	deviceOccurence := make(map[string]int)
	maxBricksPerDevice := 0
	var deviceToRemove string
	var diskNode string
	for _, brick := range vol2.Bricks {
		deviceOccurence[brick.DeviceId]++
		if deviceOccurence[brick.DeviceId] > maxBricksPerDevice {
			maxBricksPerDevice = deviceOccurence[brick.DeviceId]
			deviceToRemove = brick.DeviceId
			diskNode = brick.NodeId
		}
	}

	for device := range deviceOccurence {
		logger.Info("Key: %v , Value: %v", device, deviceOccurence[device])
	}

	// if this fails, it's a problem with the test ...
	tests.Assert(t, maxBricksPerDevice > 1, "Problem: failed to produce a disk with multiple bricks from one volume!")

	// Add a replacement disk
	driveReq := &api.DeviceAddRequest{}
	driveReq.Name = cenv.Disks[2]
	driveReq.NodeId = diskNode
	err = heketi.DeviceAdd(driveReq)
	tests.Assert(t, err == nil, err)

	stateReq := &api.StateRequest{}
	stateReq.State = api.EntryStateOffline
	err = heketi.DeviceState(deviceToRemove, stateReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	stateReq = &api.StateRequest{}
	stateReq.State = api.EntryStateFailed
	err = heketi.DeviceState(deviceToRemove, stateReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	logger.Info("%v %v", vol1, vol2)
	// Delete volumes
	err = heketi.VolumeDelete(vol1.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	err = heketi.VolumeDelete(vol2.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestRemoveDeviceVsVolumeCreate(t *testing.T) {

	// Setup the VM storage topology
	teardownCluster(t)
	setupCluster(t, 4, 1)
	defer teardownCluster(t)

	var newDevice string
	var deviceToRemove string

	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 300
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	_, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	// Check there is only one
	volumes, err := heketi.VolumeList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volumes.Volumes) == 1)

	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, err)
	for _, cluster := range clusters.Clusters {
		clusterInfo, err := heketi.ClusterInfo(cluster)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)

		for _, node := range clusterInfo.Nodes {

			// Get node information
			nodeInfo, err := heketi.NodeInfo(node)
			tests.Assert(t, err == nil, "expected err == nil, got:", err)
			for _, device := range nodeInfo.DevicesInfo {
				if len(device.Bricks) == 0 {
					newDevice = device.Id
				} else {
					deviceToRemove = device.Id
				}
			}
		}
	}

	stateReq := &api.StateRequest{}
	stateReq.State = api.EntryStateOffline
	err = heketi.DeviceState(deviceToRemove, stateReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	sgDeviceRemove := utils.NewStatusGroup()
	sgDeviceRemove.Add(1)
	go func() {
		defer sgDeviceRemove.Done()
		stateReq = &api.StateRequest{}
		stateReq.State = api.EntryStateFailed
		err = heketi.DeviceState(deviceToRemove, stateReq)
		sgDeviceRemove.Err(err)
	}()

	sgVolumeCreate := utils.NewStatusGroup()
	for i := 0; i < 15; i++ {
		sgVolumeCreate.Add(1)
		go func() {
			defer sgVolumeCreate.Done()
			volReq = &api.VolumeCreateRequest{}
			volReq.Size = 10
			volReq.Durability.Type = api.DurabilityReplicate
			volReq.Durability.Replicate.Replica = 3
			_, err := heketi.VolumeCreate(volReq)
			sgVolumeCreate.Err(err)
		}()
	}

	err = sgVolumeCreate.Result()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	err = sgDeviceRemove.Result()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	// At this point, we should have one brick moved to new device as a result of remove device
	// and 15 bricks created on new device as a result of 15 volume creates
	newDeviceResponse, err := heketi.DeviceInfo(newDevice)
	tests.Assert(t, len(newDeviceResponse.Bricks) == 16, "device entry not consistent")

}

func TestHeketiVolumeExpandWithGid(t *testing.T) {
	na := testutils.RequireNodeAccess(t)
	// Setup the VM storage topology
	teardownCluster(t)
	setupCluster(t, 3, 8)
	defer teardownCluster(t)

	// Create a volume
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 300
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5

	// Set to the vagrant gid
	volReq.Gid = 2333

	// Create the volume
	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// Expand volume
	volExpReq := &api.VolumeExpandRequest{}
	volExpReq.Size = 300

	newVolInfo, err := heketi.VolumeExpand(volInfo.Id, volExpReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, newVolInfo.Size == volInfo.Size+300)

	// SSH into system and check gid of bricks
	vagrantexec := na.Use(logger)
	cmd := []string{
		fmt.Sprintf("sudo ls -l /var/lib/heketi/mounts/vg_*/brick_*/  | grep  -e \"^d\" | cut -d\" \" -f4 | grep -q %v", volReq.Gid),
	}
	_, err = vagrantexec.ConnectAndExec(cenv.SshHost(0), cmd, 10, true)
	tests.Assert(t, err == nil, "Brick found with different Gid")
	_, err = vagrantexec.ConnectAndExec(cenv.SshHost(1), cmd, 10, true)
	tests.Assert(t, err == nil, "Brick found with different Gid")
	_, err = vagrantexec.ConnectAndExec(cenv.SshHost(2), cmd, 10, true)
	tests.Assert(t, err == nil, "Brick found with different Gid")
}

func TestHeketiVolumeCreateWithOptions(t *testing.T) {
	na := testutils.RequireNodeAccess(t)
	// Setup the VM storage topology
	teardownCluster(t)
	setupCluster(t, 2, 2)
	defer teardownCluster(t)

	// Create a volume
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 2
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5
	volReq.GlusterVolumeOptions = []string{"performance.rda-cache-limit 10MB"}

	// Set to the vagrant gid
	volReq.Gid = 2333

	// Create the volume
	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, len(volInfo.GlusterVolumeOptions) > 0)

	// SSH into system and check volume options.
	vagrantexec := na.Use(logger)
	cmd := []string{
		fmt.Sprintf("sudo gluster v info %v | grep performance.rda-cache-limit | grep 10MB", volInfo.Name),
	}
	_, err = vagrantexec.ConnectAndExec(cenv.SshHost(0), cmd, 10, true)
	tests.Assert(t, err == nil, "Volume Created with specified options")

}

func TestHeketiVolumeCreateSetsIdOption(t *testing.T) {
	na := testutils.RequireNodeAccess(t)
	// Setup the VM storage topology
	teardownCluster(t)
	setupCluster(t, 2, 2)
	defer teardownCluster(t)

	// Create a volume
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 10
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 2
	volReq.Snapshot.Enable = true
	volReq.Snapshot.Factor = 1.5

	// Create the volume
	volInfo, err := heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// SSH into system and check for user.heketi.id option
	exec := na.Use(logger)
	cmd := []string{
		fmt.Sprintf("sudo gluster v info %v | grep user.heketi.id | grep %v", volInfo.Name, volInfo.Id),
	}
	_, err = exec.ConnectAndExec(cenv.SshHost(0), cmd, 10, true)
	tests.Assert(t, err == nil, "Volume not created with user.heketi.id option")
}

func TestDeviceRemoveErrorHandling(t *testing.T) {
	na := testutils.RequireNodeAccess(t)
	teardownCluster(t)
	setupCluster(t, 2, 2)
	defer teardownCluster(t)

	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	clusterInfo, err := heketi.ClusterInfo(clusters.Clusters[0])
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	nodeInfo, err := heketi.NodeInfo(clusterInfo.Nodes[0])
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	deviceInfo := nodeInfo.DevicesInfo[0]

	// put device in failed state so that we can remove it
	err = heketi.DeviceState(deviceInfo.Id,
		&api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = heketi.DeviceState(deviceInfo.Id,
		&api.StateRequest{State: api.EntryStateFailed})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// place a dummy pv on the vg so that a clean vg remove is not possible
	host := nodeInfo.Hostnames.Manage[0] + ":" + cenv.SSHPort
	s := na.Use(logger)

	cmds := []string{
		"lvcreate -qq --autobackup=n --size 1024K --name TEST vg_" + deviceInfo.Id,
	}
	_, err = s.ConnectAndExec(host, cmds, 10, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// device has a dummy LV. delete fails
	err = heketi.DeviceDelete(deviceInfo.Id)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	cmds = []string{
		fmt.Sprintf("lvremove -q -f vg_%s/TEST", deviceInfo.Id),
	}
	_, err = s.ConnectAndExec(host, cmds, 10, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// device is free of LVs. delete is allowed
	err = heketi.DeviceDelete(deviceInfo.Id)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func TestDeviceRemoveForceForget(t *testing.T) {
	na := testutils.RequireNodeAccess(t)
	teardownCluster(t)
	setupCluster(t, 2, 2)
	defer teardownCluster(t)

	clusters, err := heketi.ClusterList()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	clusterInfo, err := heketi.ClusterInfo(clusters.Clusters[0])
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	nodeInfo, err := heketi.NodeInfo(clusterInfo.Nodes[0])
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	deviceInfo := nodeInfo.DevicesInfo[0]

	// put device in failed state so that we can remove it
	err = heketi.DeviceState(deviceInfo.Id,
		&api.StateRequest{State: api.EntryStateOffline})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	err = heketi.DeviceState(deviceInfo.Id,
		&api.StateRequest{State: api.EntryStateFailed})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// place a dummy pv on the vg so that a clean vg remove is not possible
	host := nodeInfo.Hostnames.Manage[0] + ":" + cenv.SSHPort
	s := na.Use(logger)

	cmds := []string{
		"lvcreate -qq --autobackup=n --size 1024K --name TEST vg_" + deviceInfo.Id,
	}
	_, err = s.ConnectAndExec(host, cmds, 10, true)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	defer func() {
		// because we dropped the device from heketi we need to remove
		// it ourselves
		cmds = []string{
			fmt.Sprintf("lvremove -q -f vg_%s/TEST", deviceInfo.Id),
			fmt.Sprintf("vgremove -ff vg_%s", deviceInfo.Id),
			fmt.Sprintf("pvremove -ff %s", deviceInfo.Name),
		}
		_, err = s.ConnectAndExec(host, cmds, 10, true)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}()

	// device has a dummy LV. delete fails
	logger.Info("Attempting normal remove")
	err = heketi.DeviceDelete(deviceInfo.Id)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	// device can be removed by setting the force-forget option
	logger.Info("Removing device with force forget")
	err = heketi.DeviceDeleteWithOptions(
		deviceInfo.Id, &api.DeviceDeleteOptions{ForceForget: true})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)

	// this should be an error 'cause the device is gone
	_, err = heketi.DeviceInfo(deviceInfo.Id)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
}

func TestVolumeCreateOneZone(t *testing.T) {
	// by default, heketi doesn't care if nodes are spread across
	// multiple zones. Verify that.

	tce := cenv.Copy()
	tce.Update()
	tce.CustomizeNodeRequest = func(i int, req *api.NodeAddRequest) {
		req.Zone = 1
	}

	tce.Teardown(t)
	tce.Setup(t, 4, 4)
	defer tce.Teardown(t)

	for i := 0; i < 3; i++ {
		volReq := &api.VolumeCreateRequest{}
		volReq.Size = 10
		volReq.Durability.Type = api.DurabilityReplicate
		volReq.Durability.Replicate.Replica = 3

		_, err := heketi.VolumeCreate(volReq)
		tests.Assert(t, err == nil, "expected err == nil, got:", err)
	}
}
