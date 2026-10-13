// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

type Reference interface {
	Reference() types.ManagedObjectReference
}

func NewReference(c *vim25.Client, e types.ManagedObjectReference) Reference {
	switch e.Type {
	case "Folder":
		return NewFolder(c, e)
	case "StoragePod":
		return &StoragePod{
			NewFolder(c, e),
		}
	case "Datacenter":
		return NewDatacenter(c, e)
	case "VirtualMachine":
		return NewVirtualMachine(c, e)
	case "VirtualApp":
		return &VirtualApp{
			NewResourcePool(c, e),
		}
	case "ComputeResource":
		return NewComputeResource(c, e)
	case "ClusterComputeResource":
		return NewClusterComputeResource(c, e)
	case "HostSystem":
		return NewHostSystem(c, e)
	case "Network":
		return NewNetwork(c, e)
	case "OpaqueNetwork":
		return NewOpaqueNetwork(c, e)
	case "ResourcePool":
		return NewResourcePool(c, e)
	case "DistributedVirtualSwitch":
		return NewDistributedVirtualSwitch(c, e)
	case "VmwareDistributedVirtualSwitch":
		return &VmwareDistributedVirtualSwitch{*NewDistributedVirtualSwitch(c, e)}
	case "DistributedVirtualPortgroup":
		return NewDistributedVirtualPortgroup(c, e)
	case "Datastore":
		return NewDatastore(c, e)
	default:
		panic("Unknown managed entity: " + e.Type)
	}
}
