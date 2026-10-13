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

type OptionManager struct {
	Common
}

func NewOptionManager(c *vim25.Client, ref types.ManagedObjectReference) *OptionManager {
	return &OptionManager{
		Common: NewCommon(c, ref),
	}
}

func (m OptionManager) Query(ctx context.Context, name string) ([]types.BaseOptionValue, error) {
	req := types.QueryOptions{
		This: m.Reference(),
		Name: name,
	}

	res, err := methods.QueryOptions(ctx, m.Client(), &req)
	if err != nil {
		return nil, err
	}

	return res.Returnval, nil
}

func (m OptionManager) Update(ctx context.Context, value []types.BaseOptionValue) error {
	req := types.UpdateOptions{
		This:         m.Reference(),
		ChangedValue: value,
	}

	_, err := methods.UpdateOptions(ctx, m.Client(), &req)
	return err
}
