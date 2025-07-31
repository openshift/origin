// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

type Network struct {
	Common
}

func NewNetwork(c *vim25.Client, ref types.ManagedObjectReference) *Network {
	return &Network{
		Common: NewCommon(c, ref),
	}
}

func (n Network) GetInventoryPath() string {
	return n.InventoryPath
}

// EthernetCardBackingInfo returns the VirtualDeviceBackingInfo for this Network
func (n Network) EthernetCardBackingInfo(ctx context.Context) (types.BaseVirtualDeviceBackingInfo, error) {
	name, err := n.ObjectName(ctx)
	if err != nil {
		return nil, err
	}

	backing := &types.VirtualEthernetCardNetworkBackingInfo{
		VirtualDeviceDeviceBackingInfo: types.VirtualDeviceDeviceBackingInfo{
			DeviceName: name,
		},
	}

	return backing, nil
}
