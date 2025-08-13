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

type HostDatastoreBrowser struct {
	Common
}

func NewHostDatastoreBrowser(c *vim25.Client, ref types.ManagedObjectReference) *HostDatastoreBrowser {
	return &HostDatastoreBrowser{
		Common: NewCommon(c, ref),
	}
}

func (b HostDatastoreBrowser) SearchDatastore(ctx context.Context, datastorePath string, searchSpec *types.HostDatastoreBrowserSearchSpec) (*Task, error) {
	req := types.SearchDatastore_Task{
		This:          b.Reference(),
		DatastorePath: datastorePath,
		SearchSpec:    searchSpec,
	}

	res, err := methods.SearchDatastore_Task(ctx, b.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(b.c, res.Returnval), nil
}

func (b HostDatastoreBrowser) SearchDatastoreSubFolders(ctx context.Context, datastorePath string, searchSpec *types.HostDatastoreBrowserSearchSpec) (*Task, error) {
	req := types.SearchDatastoreSubFolders_Task{
		This:          b.Reference(),
		DatastorePath: datastorePath,
		SearchSpec:    searchSpec,
	}

	res, err := methods.SearchDatastoreSubFolders_Task(ctx, b.c, &req)
	if err != nil {
		return nil, err
	}

	return NewTask(b.c, res.Returnval), nil
}
