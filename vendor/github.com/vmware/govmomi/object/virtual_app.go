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

type VirtualApp struct {
	*ResourcePool
}

func NewVirtualApp(c *vim25.Client, ref types.ManagedObjectReference) *VirtualApp {
	return &VirtualApp{
		ResourcePool: NewResourcePool(c, ref),
	}
}

func (p VirtualApp) CreateChildVM(ctx context.Context, config types.VirtualMachineConfigSpec, host *HostSystem) (*Task, error) {
	req := types.CreateChildVM_Task{
		This:   p.Reference(),
		Config: config,
	}

	if host != nil {
		ref := host.Reference()
		req.Host = &ref
	}

	res, err := methods.CreateChildVM_Task(ctx, p.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(p.c, res.Returnval), nil
}

func (p VirtualApp) UpdateConfig(ctx context.Context, spec types.VAppConfigSpec) error {
	req := types.UpdateVAppConfig{
		This: p.Reference(),
		Spec: spec,
	}

	_, err := methods.UpdateVAppConfig(ctx, p.c, &req)
	return err
}

func (p VirtualApp) PowerOn(ctx context.Context) (*Task, error) {
	req := types.PowerOnVApp_Task{
		This: p.Reference(),
	}

	res, err := methods.PowerOnVApp_Task(ctx, p.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(p.c, res.Returnval), nil
}

func (p VirtualApp) PowerOff(ctx context.Context, force bool) (*Task, error) {
	req := types.PowerOffVApp_Task{
		This:  p.Reference(),
		Force: force,
	}

	res, err := methods.PowerOffVApp_Task(ctx, p.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(p.c, res.Returnval), nil

}

func (p VirtualApp) Suspend(ctx context.Context) (*Task, error) {
	req := types.SuspendVApp_Task{
		This: p.Reference(),
	}

	res, err := methods.SuspendVApp_Task(ctx, p.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(p.c, res.Returnval), nil
}

func (p VirtualApp) Clone(ctx context.Context, name string, target types.ManagedObjectReference, spec types.VAppCloneSpec) (*Task, error) {
	req := types.CloneVApp_Task{
		This:   p.Reference(),
		Name:   name,
		Target: target,
		Spec:   spec,
	}

	res, err := methods.CloneVApp_Task(ctx, p.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(p.c, res.Returnval), nil
}
