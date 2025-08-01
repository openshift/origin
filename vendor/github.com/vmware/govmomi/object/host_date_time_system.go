// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"time"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
)

type HostDateTimeSystem struct {
	Common
}

func NewHostDateTimeSystem(c *vim25.Client, ref types.ManagedObjectReference) *HostDateTimeSystem {
	return &HostDateTimeSystem{
		Common: NewCommon(c, ref),
	}
}

func (s HostDateTimeSystem) UpdateConfig(ctx context.Context, config types.HostDateTimeConfig) error {
	req := types.UpdateDateTimeConfig{
		This:   s.Reference(),
		Config: config,
	}

	_, err := methods.UpdateDateTimeConfig(ctx, s.c, &req)
	return err
}

func (s HostDateTimeSystem) Update(ctx context.Context, date time.Time) error {
	req := types.UpdateDateTime{
		This:     s.Reference(),
		DateTime: date,
	}

	_, err := methods.UpdateDateTime(ctx, s.c, &req)
	return err
}

func (s HostDateTimeSystem) Query(ctx context.Context) (*time.Time, error) {
	req := types.QueryDateTime{
		This: s.Reference(),
	}

	res, err := methods.QueryDateTime(ctx, s.c, &req)
	if err != nil {
		return nil, err
	}

	return &res.Returnval, nil
}
