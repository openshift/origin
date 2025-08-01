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

type HostAccountManager struct {
	Common
}

func NewHostAccountManager(c *vim25.Client, ref types.ManagedObjectReference) *HostAccountManager {
	return &HostAccountManager{
		Common: NewCommon(c, ref),
	}
}

func (m HostAccountManager) Create(ctx context.Context, user *types.HostAccountSpec) error {
	req := types.CreateUser{
		This: m.Reference(),
		User: user,
	}

	_, err := methods.CreateUser(ctx, m.Client(), &req)
	return err
}

func (m HostAccountManager) Update(ctx context.Context, user *types.HostAccountSpec) error {
	req := types.UpdateUser{
		This: m.Reference(),
		User: user,
	}

	_, err := methods.UpdateUser(ctx, m.Client(), &req)
	return err
}

func (m HostAccountManager) Remove(ctx context.Context, userName string) error {
	req := types.RemoveUser{
		This:     m.Reference(),
		UserName: userName,
	}

	_, err := methods.RemoveUser(ctx, m.Client(), &req)
	return err
}
