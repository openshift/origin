// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
)

type HostDatastoreSystem struct {
	Common
}

func NewHostDatastoreSystem(c *vim25.Client, ref types.ManagedObjectReference) *HostDatastoreSystem {
	return &HostDatastoreSystem{
		Common: NewCommon(c, ref),
	}
}

func (s HostDatastoreSystem) CreateLocalDatastore(ctx context.Context, name string, path string) (*Datastore, error) {
	req := types.CreateLocalDatastore{
		This: s.Reference(),
		Name: name,
		Path: path,
	}

	res, err := methods.CreateLocalDatastore(ctx, s.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewDatastore(s.Client(), res.Returnval), nil
}

func (s HostDatastoreSystem) CreateNasDatastore(ctx context.Context, spec types.HostNasVolumeSpec) (*Datastore, error) {
	req := types.CreateNasDatastore{
		This: s.Reference(),
		Spec: spec,
	}

	res, err := methods.CreateNasDatastore(ctx, s.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewDatastore(s.Client(), res.Returnval), nil
}

func (s HostDatastoreSystem) CreateVmfsDatastore(ctx context.Context, spec types.VmfsDatastoreCreateSpec) (*Datastore, error) {
	req := types.CreateVmfsDatastore{
		This: s.Reference(),
		Spec: spec,
	}

	res, err := methods.CreateVmfsDatastore(ctx, s.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewDatastore(s.Client(), res.Returnval), nil
}

func (s HostDatastoreSystem) Remove(ctx context.Context, ds *Datastore) error {
	req := types.RemoveDatastore{
		This:      s.Reference(),
		Datastore: ds.Reference(),
	}

	_, err := methods.RemoveDatastore(ctx, s.Client(), &req)
	if err != nil {
		return err
	}

	return nil
}

func (s HostDatastoreSystem) QueryAvailableDisksForVmfs(ctx context.Context) ([]types.HostScsiDisk, error) {
	req := types.QueryAvailableDisksForVmfs{
		This: s.Reference(),
	}

	res, err := methods.QueryAvailableDisksForVmfs(ctx, s.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}

func (s HostDatastoreSystem) QueryVmfsDatastoreCreateOptions(ctx context.Context, devicePath string) ([]types.VmfsDatastoreOption, error) {
	req := types.QueryVmfsDatastoreCreateOptions{
		This:       s.Reference(),
		DevicePath: devicePath,
	}

	res, err := methods.QueryVmfsDatastoreCreateOptions(ctx, s.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}

func (s HostDatastoreSystem) ResignatureUnresolvedVmfsVolumes(ctx context.Context, devicePaths []string) (*Task, error) {
	req := &types.ResignatureUnresolvedVmfsVolume_Task{
		This: s.Reference(),
		ResolutionSpec: types.HostUnresolvedVmfsResignatureSpec{
			ExtentDevicePath: devicePaths,
		},
	}

	res, err := methods.ResignatureUnresolvedVmfsVolume_Task(ctx, s.Client(), req)
	if err != nil {
		return nil, err
	}

	return NewTask(s.c, res.Returnval), nil
}
