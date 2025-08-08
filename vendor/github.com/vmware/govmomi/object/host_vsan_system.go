// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type HostVsanSystem struct {
	Common
}

func NewHostVsanSystem(c *vim25.Client, ref types.ManagedObjectReference) *HostVsanSystem {
	return &HostVsanSystem{
		Common: NewCommon(c, ref),
	}
}

func (s HostVsanSystem) Update(ctx context.Context, config types.VsanHostConfigInfo) (*Task, error) {
	req := types.UpdateVsan_Task{
		This:   s.Reference(),
		Config: config,
	}

	res, err := methods.UpdateVsan_Task(ctx, s.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewTask(s.Client(), res.Returnval), nil
}

// updateVnic in support of the HostVirtualNicManager.{SelectVnic,DeselectVnic} methods
func (s HostVsanSystem) updateVnic(ctx context.Context, device string, enable bool) error {
	var vsan mo.HostVsanSystem

	err := s.Properties(ctx, s.Reference(), []string{"config.networkInfo.port"}, &vsan)
	if err != nil {
		return err
	}

	info := vsan.Config

	var port []types.VsanHostConfigInfoNetworkInfoPortConfig

	for _, p := range info.NetworkInfo.Port {
		if p.Device == device {
			continue
		}

		port = append(port, p)
	}

	if enable {
		port = append(port, types.VsanHostConfigInfoNetworkInfoPortConfig{
			Device: device,
		})
	}

	info.NetworkInfo.Port = port

	task, err := s.Update(ctx, info)
	if err != nil {
		return err
	}

	_, err = task.WaitForResult(ctx, nil)
	return err
}
