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

type Manager struct {
	object.Common
}

func NewManager(c *vim25.Client) *Manager {
	m := Manager{
		object.NewCommon(c, *c.ServiceContent.ViewManager),
	}

	return &m
}

func (m Manager) CreateListView(ctx context.Context, objects []types.ManagedObjectReference) (*ListView, error) {
	req := types.CreateListView{
		This: m.Common.Reference(),
		Obj:  objects,
	}

	res, err := methods.CreateListView(ctx, m.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewListView(m.Client(), res.Returnval), nil
}

func (m Manager) CreateContainerView(ctx context.Context, container types.ManagedObjectReference, managedObjectTypes []string, recursive bool) (*ContainerView, error) {

	req := types.CreateContainerView{
		This:      m.Common.Reference(),
		Container: container,
		Recursive: recursive,
		Type:      managedObjectTypes,
	}

	res, err := methods.CreateContainerView(ctx, m.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewContainerView(m.Client(), res.Returnval), nil
}
