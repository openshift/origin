// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type DistributedVirtualPortgroup struct {
	Common
}

func NewDistributedVirtualPortgroup(c *vim25.Client, ref types.ManagedObjectReference) *DistributedVirtualPortgroup {
	return &DistributedVirtualPortgroup{
		Common: NewCommon(c, ref),
	}
}

func (p DistributedVirtualPortgroup) GetInventoryPath() string {
	return p.InventoryPath
}

// EthernetCardBackingInfo returns the VirtualDeviceBackingInfo for this DistributedVirtualPortgroup
func (p DistributedVirtualPortgroup) EthernetCardBackingInfo(ctx context.Context) (types.BaseVirtualDeviceBackingInfo, error) {
	var dvp mo.DistributedVirtualPortgroup
	var dvs mo.DistributedVirtualSwitch
	prop := "config.distributedVirtualSwitch"

	if err := p.Properties(ctx, p.Reference(), []string{"key", prop}, &dvp); err != nil {
		return nil, err
	}

	// From the docs at https://developer.broadcom.com/xapis/vsphere-web-services-api/latest/vim.dvs.DistributedVirtualPortgroup.ConfigInfo.html:
	// "This property should always be set unless the user's setting does not have System.Read privilege on the object referred to by this property."
	// Note that "the object" refers to the Switch, not the PortGroup.
	if dvp.Config.DistributedVirtualSwitch == nil {
		name := p.InventoryPath
		if name == "" {
			name = p.Reference().String()
		}
		return nil, fmt.Errorf("failed to create EthernetCardBackingInfo for %s: System.Read privilege required for %s", name, prop)
	}

	if err := p.Properties(ctx, *dvp.Config.DistributedVirtualSwitch, []string{"uuid"}, &dvs); err != nil {
		return nil, err
	}

	backing := &types.VirtualEthernetCardDistributedVirtualPortBackingInfo{
		Port: types.DistributedVirtualSwitchPortConnection{
			PortgroupKey: dvp.Key,
			SwitchUuid:   dvs.Uuid,
		},
	}

	return backing, nil
}

func (p DistributedVirtualPortgroup) Reconfigure(ctx context.Context, spec types.DVPortgroupConfigSpec) (*Task, error) {
	req := types.ReconfigureDVPortgroup_Task{
		This: p.Reference(),
		Spec: spec,
	}

	res, err := methods.ReconfigureDVPortgroup_Task(ctx, p.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewTask(p.Client(), res.Returnval), nil
}
