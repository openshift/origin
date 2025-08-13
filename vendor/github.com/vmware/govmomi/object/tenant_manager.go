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

type TenantManager struct {
	Common
}

func NewTenantManager(c *vim25.Client) *TenantManager {
	t := TenantManager{
		Common: NewCommon(c, *c.ServiceContent.TenantManager),
	}

	return &t
}

func (t TenantManager) MarkServiceProviderEntities(ctx context.Context, entities []types.ManagedObjectReference) error {
	req := types.MarkServiceProviderEntities{
		This:   t.Reference(),
		Entity: entities,
	}

	_, err := methods.MarkServiceProviderEntities(ctx, t.Client(), &req)
	if err != nil {
		return err
	}

	return nil
}

func (t TenantManager) UnmarkServiceProviderEntities(ctx context.Context, entities []types.ManagedObjectReference) error {
	req := types.UnmarkServiceProviderEntities{
		This:   t.Reference(),
		Entity: entities,
	}

	_, err := methods.UnmarkServiceProviderEntities(ctx, t.Client(), &req)
	if err != nil {
		return err
	}

	return nil
}

func (t TenantManager) RetrieveServiceProviderEntities(ctx context.Context) ([]types.ManagedObjectReference, error) {
	req := types.RetrieveServiceProviderEntities{
		This: t.Reference(),
	}

	res, err := methods.RetrieveServiceProviderEntities(ctx, t.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}
