// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"context"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
)

type ListView struct {
	ManagedObjectView
}

func NewListView(c *vim25.Client, ref types.ManagedObjectReference) *ListView {
	return &ListView{
		ManagedObjectView: *NewManagedObjectView(c, ref),
	}
}

func (v ListView) Add(ctx context.Context, refs []types.ManagedObjectReference) ([]types.ManagedObjectReference, error) {
	req := types.ModifyListView{
		This: v.Reference(),
		Add:  refs,
	}
	res, err := methods.ModifyListView(ctx, v.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}

func (v ListView) Remove(ctx context.Context, refs []types.ManagedObjectReference) ([]types.ManagedObjectReference, error) {
	req := types.ModifyListView{
		This:   v.Reference(),
		Remove: refs,
	}
	res, err := methods.ModifyListView(ctx, v.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}

func (v ListView) Reset(ctx context.Context, refs []types.ManagedObjectReference) ([]types.ManagedObjectReference, error) {
	req := types.ResetListView{
		This: v.Reference(),
		Obj:  refs,
	}
	res, err := methods.ResetListView(ctx, v.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}
