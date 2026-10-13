// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

type VmwareDistributedVirtualSwitch struct {
	DistributedVirtualSwitch
}

func (s VmwareDistributedVirtualSwitch) GetInventoryPath() string {
	return s.InventoryPath
}
