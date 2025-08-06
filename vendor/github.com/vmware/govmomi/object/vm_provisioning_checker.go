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

// VmProvisioningChecker models the ProvisioningChecker, a singleton managed
// object that can answer questions about the feasibility of certain
// provisioning operations.
//
// For more information, see:
// https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.check.ProvisioningChecker.html
type VmProvisioningChecker struct {
	Common
}

func NewVmProvisioningChecker(c *vim25.Client) *VmProvisioningChecker {
	return &VmProvisioningChecker{
		Common: NewCommon(c, *c.ServiceContent.VmProvisioningChecker),
	}
}

func (c VmProvisioningChecker) CheckRelocate(
	ctx context.Context,
	vm types.ManagedObjectReference,
	spec types.VirtualMachineRelocateSpec,
	testTypes ...types.CheckTestType) ([]types.CheckResult, error) {

	req := types.CheckRelocate_Task{
		This:     c.Reference(),
		Vm:       vm,
		Spec:     spec,
		TestType: checkTestTypesToStrings(testTypes),
	}

	res, err := methods.CheckRelocate_Task(ctx, c.c, &req)
	if err != nil {
		return nil, err
	}

	ti, err := NewTask(c.c, res.Returnval).WaitForResult(ctx)
	if err != nil {
		return nil, err
	}

	return ti.Result.(types.ArrayOfCheckResult).CheckResult, nil
}
