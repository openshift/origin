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

type EnvironmentBrowser struct {
	Common
}

func NewEnvironmentBrowser(c *vim25.Client, ref types.ManagedObjectReference) *EnvironmentBrowser {
	return &EnvironmentBrowser{
		Common: NewCommon(c, ref),
	}
}

func (b EnvironmentBrowser) QueryConfigTarget(ctx context.Context, host *HostSystem) (*types.ConfigTarget, error) {
	req := types.QueryConfigTarget{
		This: b.Reference(),
	}

	if host != nil {
		ref := host.Reference()
		req.Host = &ref
	}

	res, err := methods.QueryConfigTarget(ctx, b.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}

func (b EnvironmentBrowser) QueryTargetCapabilities(ctx context.Context, host *HostSystem) (*types.HostCapability, error) {
	req := types.QueryTargetCapabilities{
		This: b.Reference(),
	}

	if host != nil {
		ref := host.Reference()
		req.Host = &ref
	}

	res, err := methods.QueryTargetCapabilities(ctx, b.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}

func (b EnvironmentBrowser) QueryConfigOption(ctx context.Context, spec *types.EnvironmentBrowserConfigOptionQuerySpec) (*types.VirtualMachineConfigOption, error) {
	req := types.QueryConfigOptionEx{
		This: b.Reference(),
		Spec: spec,
	}

	res, err := methods.QueryConfigOptionEx(ctx, b.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}

func (b EnvironmentBrowser) QueryConfigOptionDescriptor(ctx context.Context) ([]types.VirtualMachineConfigOptionDescriptor, error) {
	req := types.QueryConfigOptionDescriptor{
		This: b.Reference(),
	}

	res, err := methods.QueryConfigOptionDescriptor(ctx, b.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}
