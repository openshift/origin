// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"context"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
)

type ManagedObjectView struct {
	object.Common
}

func NewManagedObjectView(c *vim25.Client, ref types.ManagedObjectReference) *ManagedObjectView {
	return &ManagedObjectView{
		Common: object.NewCommon(c, ref),
	}
}

func (v *ManagedObjectView) TraversalSpec() *types.TraversalSpec {
	return &types.TraversalSpec{
		Path: "view",
		Type: v.Reference().Type,
	}
}

func (v *ManagedObjectView) Destroy(ctx context.Context) error {
	req := types.DestroyView{
		This: v.Reference(),
	}

	_, err := methods.DestroyView(ctx, v.Client(), &req)
	return err
}
