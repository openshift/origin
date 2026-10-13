// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"fmt"
	"net"

	"github.com/vmware/govmomi/internal"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type HostSystem struct {
	Common
}

func NewHostSystem(c *vim25.Client, ref types.ManagedObjectReference) *HostSystem {
	return &HostSystem{
		Common: NewCommon(c, ref),
	}
}

func (h HostSystem) ConfigManager() *HostConfigManager {
	return NewHostConfigManager(h.c, h.Reference())
}

func (h HostSystem) ResourcePool(ctx context.Context) (*ResourcePool, error) {
	var mh mo.HostSystem

	err := h.Properties(ctx, h.Reference(), []string{"parent"}, &mh)
	if err != nil {
		return nil, err
	}

	var mcr *mo.ComputeResource
	var parent any

	switch mh.Parent.Type {
	case "ComputeResource":
		mcr = new(mo.ComputeResource)
		parent = mcr
	case "ClusterComputeResource":
		mcc := new(mo.ClusterComputeResource)
		mcr = &mcc.ComputeResource
		parent = mcc
	default:
		return nil, fmt.Errorf("unknown host parent type: %s", mh.Parent.Type)
	}

	err = h.Properties(ctx, *mh.Parent, []string{"resourcePool"}, parent)
	if err != nil {
		return nil, err
	}

	pool := NewResourcePool(h.c, *mcr.ResourcePool)
	return pool, nil
}

func (h HostSystem) ManagementIPs(ctx context.Context) ([]net.IP, error) {
	var mh mo.HostSystem

	err := h.Properties(ctx, h.Reference(), []string{"config.virtualNicManagerInfo.netConfig"}, &mh)
	if err != nil {
		return nil, err
	}

	config := mh.Config
	if config == nil {
		return nil, nil
	}

	info := config.VirtualNicManagerInfo
	if info == nil {
		return nil, nil
	}

	return internal.HostSystemManagementIPs(info.NetConfig), nil
}

func (h HostSystem) Disconnect(ctx context.Context) (*Task, error) {
	req := types.DisconnectHost_Task{
		This: h.Reference(),
	}

	res, err := methods.DisconnectHost_Task(ctx, h.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(h.c, res.Returnval), nil
}

func (h HostSystem) Reconnect(ctx context.Context, cnxSpec *types.HostConnectSpec, reconnectSpec *types.HostSystemReconnectSpec) (*Task, error) {
	req := types.ReconnectHost_Task{
		This:          h.Reference(),
		CnxSpec:       cnxSpec,
		ReconnectSpec: reconnectSpec,
	}

	res, err := methods.ReconnectHost_Task(ctx, h.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(h.c, res.Returnval), nil
}

func (h HostSystem) EnterMaintenanceMode(ctx context.Context, timeout int32, evacuate bool, spec *types.HostMaintenanceSpec) (*Task, error) {
	req := types.EnterMaintenanceMode_Task{
		This:                  h.Reference(),
		Timeout:               timeout,
		EvacuatePoweredOffVms: types.NewBool(evacuate),
		MaintenanceSpec:       spec,
	}

	res, err := methods.EnterMaintenanceMode_Task(ctx, h.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(h.c, res.Returnval), nil
}

func (h HostSystem) ExitMaintenanceMode(ctx context.Context, timeout int32) (*Task, error) {
	req := types.ExitMaintenanceMode_Task{
		This:    h.Reference(),
		Timeout: timeout,
	}

	res, err := methods.ExitMaintenanceMode_Task(ctx, h.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(h.c, res.Returnval), nil
}
