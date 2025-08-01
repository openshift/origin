// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type OpaqueNetwork struct {
	Common
}

func NewOpaqueNetwork(c *vim25.Client, ref types.ManagedObjectReference) *OpaqueNetwork {
	return &OpaqueNetwork{
		Common: NewCommon(c, ref),
	}
}

func (n OpaqueNetwork) GetInventoryPath() string {
	return n.InventoryPath
}

// EthernetCardBackingInfo returns the VirtualDeviceBackingInfo for this Network
func (n OpaqueNetwork) EthernetCardBackingInfo(ctx context.Context) (types.BaseVirtualDeviceBackingInfo, error) {
	summary, err := n.Summary(ctx)
	if err != nil {
		return nil, err
	}

	backing := &types.VirtualEthernetCardOpaqueNetworkBackingInfo{
		OpaqueNetworkId:   summary.OpaqueNetworkId,
		OpaqueNetworkType: summary.OpaqueNetworkType,
	}

	return backing, nil
}

// Summary returns the mo.OpaqueNetwork.Summary property
func (n OpaqueNetwork) Summary(ctx context.Context) (*types.OpaqueNetworkSummary, error) {
	var props mo.OpaqueNetwork

	err := n.Properties(ctx, n.Reference(), []string{"summary"}, &props)
	if err != nil {
		return nil, err
	}

	summary, ok := props.Summary.(*types.OpaqueNetworkSummary)
	if !ok {
		return nil, fmt.Errorf("%s unsupported network summary type: %T", n, props.Summary)
	}

	return summary, nil
}
