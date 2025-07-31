// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"

	"github.com/vmware/govmomi/vim25/types"
)

// The NetworkReference interface is implemented by managed objects
// which can be used as the backing for a VirtualEthernetCard.
type NetworkReference interface {
	Reference
	GetInventoryPath() string
	EthernetCardBackingInfo(ctx context.Context) (types.BaseVirtualDeviceBackingInfo, error)
}
