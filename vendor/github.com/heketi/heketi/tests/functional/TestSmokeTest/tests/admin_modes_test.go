// +build functional

//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package functional

import (
	"os"
	"strconv"
	"syscall"
	"testing"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/tests"
)

func TestAdminModes(t *testing.T) {
	setupCluster(t, 4, 8)
	defer teardownCluster(t)
	t.Run("testAdminStatusGet", testAdminStatusGet)
	teardownVolumes(t)
	t.Run("testAdminStatusLocal", testAdminStatusLocal)
	teardownVolumes(t)
	t.Run("testAdminStatusNormal", testAdminStatusNormal)
	teardownVolumes(t)
	t.Run("testAdminStatusReadOnly", testAdminStatusReadOnly)
	t.Run("testAdminStatusReset", testAdminStatusReset)
}

func checkAdminState(t *testing.T, s api.AdminState) {
	as, err := heketi.AdminStatusGet()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, as.State == s,
		"expected as.State == s, got:", as.State, s)
}

func testAdminStatusGet(t *testing.T) {
	checkAdminState(t, api.AdminStateNormal)
}

func testAdminStatusLocal(t *testing.T) {
	err := heketi.AdminStatusSet(&api.AdminStatus{
		State: api.AdminStateLocal,
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	checkAdminState(t, api.AdminStateLocal)

	// now we should still be able to create a volume as
	// the test runs on the same host as heketi
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	_, err = heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func testAdminStatusNormal(t *testing.T) {
	testAdminStatusLocal(t)
	// re-executing the local status test will leave this
	// in local-only mode. But because we are a local client
	// we should be able to set server back to normal

	err := heketi.AdminStatusSet(&api.AdminStatus{
		State: api.AdminStateNormal,
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	checkAdminState(t, api.AdminStateNormal)

	// now we should still be able to create a volume as
	// the test runs on the same host as heketi
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	_, err = heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

func testAdminStatusReadOnly(t *testing.T) {
	err := heketi.AdminStatusSet(&api.AdminStatus{
		State: api.AdminStateReadOnly,
	})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	checkAdminState(t, api.AdminStateReadOnly)

	// now we can no longer make changes to the system
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	_, err = heketi.VolumeCreate(volReq)
	tests.Assert(t, err != nil, "expected err != nil, got:", err)

	// we can not even change the admin mode
	err = heketi.AdminStatusSet(&api.AdminStatus{
		State: api.AdminStateNormal,
	})
	tests.Assert(t, err != nil, "expected err != nil, got:", err)
}

func testAdminStatusReset(t *testing.T) {
	as, err := heketi.AdminStatusGet()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	if as.State != api.AdminStateReadOnly {
		testAdminStatusReadOnly(t)
	}

	checkAdminState(t, api.AdminStateReadOnly)
	syscall.Kill(heketiPid(), syscall.SIGUSR2)
	checkAdminState(t, api.AdminStateNormal)

	// now we should be able to create a volume again
	volReq := &api.VolumeCreateRequest{}
	volReq.Size = 1
	volReq.Durability.Type = api.DurabilityReplicate
	volReq.Durability.Replicate.Replica = 3
	_, err = heketi.VolumeCreate(volReq)
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}

// heketiPid returns the pid of the current heketi process.
// If the pid can not be determined the function will panic.
func heketiPid() int {
	hpid := os.Getenv("HEKETI_PID")
	if hpid == "" {
		panic("no heketi pid supplied")
	}
	i, err := strconv.Atoi(hpid)
	if err != nil {
		panic(err)
	}
	return i
}
