package nutanix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
	v2 "github.com/tecbiz-ch/nutanix-go-sdk/schema/v2"
)

const (
	vmSnapshotv2Path         = "/snapshots"
	vmSnapshotv2VMSinglePath = vmSnapshotv2Path + "?vm_uuid=%s"
	vmSnapshotv2SinglePath   = vmSnapshotv2Path + "/%s"
	vmSnapshotv2RestorePath  = vmSinglePath + "/restore"
)

// SnapshotClient ... is a client for the vm API.
type SnapshotClient struct {
	client *Client
}

// ListByVM ...
func (c *SnapshotClient) ListByVM(ctx context.Context, vm *schema.VMIntent) (*v2.SnapshotList, error) {
	filter := &v2.Metadata{FilterCriteria: fmt.Sprintf("vm_uuid==%s", vm.Metadata.UUID)}
	return c.List(ctx, filter, vm)
}

// Get ...
func (c *SnapshotClient) Get(ctx context.Context, idOrName string, vm *schema.VMIntent) (*v2.SnapshotSpec, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName, vm)
	}
	return c.GetByName(ctx, idOrName, vm)
}

// GetByUUID retrieves an vm by its UUID. If the vm does not exist, nil is returned.
func (c *SnapshotClient) GetByUUID(ctx context.Context, uuid string, vm *schema.VMIntent) (*v2.SnapshotSpec, error) {
	req, err := c.client.NewV2PERequest(ctx, "GET", vm.Spec.ClusterReference.UUID, fmt.Sprintf(vmSnapshotv2SinglePath, uuid), nil)
	if err != nil {
		return nil, err
	}

	snapshot := new(v2.SnapshotSpec)
	err = c.client.Do(req, &snapshot)
	if err != nil {
		return nil, err
	}
	return snapshot, nil

}

// GetByName retrieves an vm by its name. If the vm does not exist, nil is returned.
func (c *SnapshotClient) GetByName(ctx context.Context, name string, vm *schema.VMIntent) (*v2.SnapshotSpec, error) {
	_ = fmt.Sprintf(vmSnapshotv2SinglePath, vm.Metadata.UUID)
	// filter := &v2.Metadata{FilterCriteria: utils.StringPtr(fmt.Sprintf("name==%s", name))}
	filter := &v2.Metadata{}

	list, err := c.List(ctx, filter, vm)
	if len(list.Entities) == 0 {
		return nil, fmt.Errorf("VM Snapshot not found: %s", name)
	}
	return list.Entities[0], err
}

// List ...
func (c *SnapshotClient) List(ctx context.Context, opts *v2.Metadata, vm *schema.VMIntent) (*v2.SnapshotList, error) {
	path := fmt.Sprintf(vmSnapshotv2VMSinglePath, vm.Metadata.UUID)
	reqBodyData, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	req, err := c.client.NewV2PERequest(ctx, http.MethodGet, vm.Spec.ClusterReference.UUID, path, bytes.NewReader(reqBodyData))
	if err != nil {
		return nil, err
	}

	list := new(v2.SnapshotList)
	err = c.client.Do(req, &list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (c *SnapshotClient) Restore(ctx context.Context, snapshot *v2.SnapshotSpec, vm *schema.VMIntent) (*v2.Task, error) {
	snapshotRestore := &v2.SnapshotRestore{
		UUID:                        vm.Metadata.UUID,
		SnapshotUUID:                snapshot.UUID,
		RestoreNetworkConfiguration: true,
	}
	reqBodyData, err := json.Marshal(snapshotRestore)
	if err != nil {
		return nil, err
	}

	req, err := c.client.NewV2PERequest(ctx, http.MethodPost, vm.Spec.ClusterReference.UUID, fmt.Sprintf(vmSnapshotv2RestorePath, vm.Metadata.UUID), bytes.NewReader(reqBodyData))
	if err != nil {
		return nil, err
	}

	response := new(v2.Task)

	err = c.client.Do(req, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// Create ...
func (c *SnapshotClient) Create(ctx context.Context, name string, vm *schema.VMIntent) (*v2.Task, error) {
	snapspec := &v2.SnapshotSpec{
		VMUUID: vm.Metadata.UUID,
		Name:   name,
	}

	snapshot := new(v2.SnapshotCreate)
	snapshot.Entities = append(snapshot.Entities, snapspec)

	reqBodyData, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}

	req, err := c.client.NewV2PERequest(ctx, http.MethodPost, vm.Spec.ClusterReference.UUID, vmSnapshotv2Path, bytes.NewReader(reqBodyData))
	if err != nil {
		return nil, err
	}
	response := new(v2.Task)
	err = c.client.Do(req, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// Delete ...
func (c *SnapshotClient) Delete(ctx context.Context, vm *schema.VMIntent, snapshot *v2.SnapshotSpec) (*v2.Task, error) {
	response := new(v2.Task)
	path := fmt.Sprintf(vmSnapshotv2SinglePath, snapshot.UUID)
	req, err := c.client.NewV2PERequest(ctx, http.MethodDelete, vm.Spec.ClusterReference.UUID, path, nil)
	if err != nil {
		return response, err
	}

	err = c.client.Do(req, &response)
	if err != nil {
		return response, err
	}
	return response, nil
}
