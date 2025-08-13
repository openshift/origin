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

type DatastoreNamespaceManager struct {
	Common
}

func NewDatastoreNamespaceManager(c *vim25.Client) *DatastoreNamespaceManager {
	n := DatastoreNamespaceManager{
		Common: NewCommon(c, *c.ServiceContent.DatastoreNamespaceManager),
	}

	return &n
}

// CreateDirectory creates a top-level directory on the given vsan datastore, using
// the given user display name hint and opaque storage policy.
func (nm DatastoreNamespaceManager) CreateDirectory(ctx context.Context, ds *Datastore, displayName string, policy string) (string, error) {

	req := &types.CreateDirectory{
		This:        nm.Reference(),
		Datastore:   ds.Reference(),
		DisplayName: displayName,
		Policy:      policy,
	}

	resp, err := methods.CreateDirectory(ctx, nm.c, req)
	if err != nil {
		return "", err
	}

	return resp.Returnval, nil
}

// DeleteDirectory deletes the given top-level directory from a vsan datastore.
func (nm DatastoreNamespaceManager) DeleteDirectory(ctx context.Context, dc *Datacenter, datastorePath string) error {

	req := &types.DeleteDirectory{
		This:          nm.Reference(),
		DatastorePath: datastorePath,
	}

	if dc != nil {
		ref := dc.Reference()
		req.Datacenter = &ref
	}

	if _, err := methods.DeleteDirectory(ctx, nm.c, req); err != nil {
		return err
	}

	return nil
}

func (nm DatastoreNamespaceManager) ConvertNamespacePathToUuidPath(ctx context.Context, dc *Datacenter, datastoreURL string) (string, error) {
	req := &types.ConvertNamespacePathToUuidPath{
		This:         nm.Reference(),
		NamespaceUrl: datastoreURL,
	}

	if dc != nil {
		ref := dc.Reference()
		req.Datacenter = &ref
	}

	res, err := methods.ConvertNamespacePathToUuidPath(ctx, nm.c, req)
	if err != nil {
		return "", err
	}

	return res.Returnval, nil
}
