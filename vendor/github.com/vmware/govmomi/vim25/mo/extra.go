// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package mo

type IsManagedEntity interface {
	GetManagedEntity() ManagedEntity
}

func (m ComputeResource) GetManagedEntity() ManagedEntity {
	return m.ManagedEntity
}

func (m Datacenter) GetManagedEntity() ManagedEntity {
	return m.ManagedEntity
}

func (m Datastore) GetManagedEntity() ManagedEntity {
	return m.ManagedEntity
}

func (m DistributedVirtualSwitch) GetManagedEntity() ManagedEntity {
	return m.ManagedEntity
}

func (m DistributedVirtualPortgroup) GetManagedEntity() ManagedEntity {
	return m.ManagedEntity
}

func (m Folder) GetManagedEntity() ManagedEntity {
	return m.ManagedEntity
}

func (m HostSystem) GetManagedEntity() ManagedEntity {
	return m.ManagedEntity
}

func (m Network) GetManagedEntity() ManagedEntity {
	return m.ManagedEntity
}

func (m ResourcePool) GetManagedEntity() ManagedEntity {
	return m.ManagedEntity
}

func (m VirtualMachine) GetManagedEntity() ManagedEntity {
	return m.ManagedEntity
}
