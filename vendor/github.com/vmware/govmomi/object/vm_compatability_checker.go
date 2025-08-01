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

// VmCompatibilityChecker models the CompatibilityChecker, a singleton managed
// object that can answer questions about compatibility of a virtual machine
// with a host.
//
// For more information, see:
// https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.check.CompatibilityChecker.html
type VmCompatibilityChecker struct {
	Common
}

func NewVmCompatibilityChecker(c *vim25.Client) *VmCompatibilityChecker {
	return &VmCompatibilityChecker{
		Common: NewCommon(c, *c.ServiceContent.VmCompatibilityChecker),
	}
}

func (c VmCompatibilityChecker) CheckCompatibility(
	ctx context.Context,
	vm types.ManagedObjectReference,
	host *types.ManagedObjectReference,
	pool *types.ManagedObjectReference,
	testTypes ...types.CheckTestType) ([]types.CheckResult, error) {

	req := types.CheckCompatibility_Task{
		This:     c.Reference(),
		Vm:       vm,
		Host:     host,
		Pool:     pool,
		TestType: checkTestTypesToStrings(testTypes),
	}

	res, err := methods.CheckCompatibility_Task(ctx, c.c, &req)
	if err != nil {
		return nil, err
	}

	ti, err := NewTask(c.c, res.Returnval).WaitForResult(ctx)
	if err != nil {
		return nil, err
	}

	return ti.Result.(types.ArrayOfCheckResult).CheckResult, nil
}

func (c VmCompatibilityChecker) CheckVmConfig(
	ctx context.Context,
	spec types.VirtualMachineConfigSpec,
	vm *types.ManagedObjectReference,
	host *types.ManagedObjectReference,
	pool *types.ManagedObjectReference,
	testTypes ...types.CheckTestType) ([]types.CheckResult, error) {

	req := types.CheckVmConfig_Task{
		This:     c.Reference(),
		Spec:     spec,
		Vm:       vm,
		Host:     host,
		Pool:     pool,
		TestType: checkTestTypesToStrings(testTypes),
	}

	res, err := methods.CheckVmConfig_Task(ctx, c.c, &req)
	if err != nil {
		return nil, err
	}

	ti, err := NewTask(c.c, res.Returnval).WaitForResult(ctx)
	if err != nil {
		return nil, err
	}

	return ti.Result.(types.ArrayOfCheckResult).CheckResult, nil
}

func checkTestTypesToStrings(testTypes []types.CheckTestType) []string {
	if len(testTypes) == 0 {
		return nil
	}

	s := make([]string, len(testTypes))
	for i := range testTypes {
		s[i] = string(testTypes[i])
	}
	return s
}
