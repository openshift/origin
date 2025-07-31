// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
)

type Manager struct {
	r types.ManagedObjectReference
	c *vim25.Client
}

// NewManager creates a new task manager
func NewManager(c *vim25.Client) *Manager {
	m := Manager{
		r: *c.ServiceContent.TaskManager,
		c: c,
	}

	return &m
}

// Reference returns the task.Manager MOID
func (m Manager) Reference() types.ManagedObjectReference {
	return m.r
}

// CreateCollectorForTasks returns a task history collector, a specialized
// history collector that gathers TaskInfo data objects.
func (m Manager) CreateCollectorForTasks(ctx context.Context, filter types.TaskFilterSpec) (*HistoryCollector, error) {
	req := types.CreateCollectorForTasks{
		This:   m.r,
		Filter: filter,
	}

	res, err := methods.CreateCollectorForTasks(ctx, m.c, &req)
	if err != nil {
		return nil, err
	}

	return newHistoryCollector(m.c, res.Returnval), nil
}
