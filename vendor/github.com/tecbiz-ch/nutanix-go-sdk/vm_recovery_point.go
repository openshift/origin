package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	vmRecoveryPointBasePath   = "/vm_recovery_points"
	vmRecoveryPointListPath   = vmRecoveryPointBasePath + "/list"
	vmRecoveryPointSinglePath = vmRecoveryPointBasePath + "/%s"
)

// VMRecoveryPointClient is a client for the vm API.
type VMRecoveryPointClient struct {
	client *Client
}

// Get ...
func (c *VMRecoveryPointClient) Get(ctx context.Context, idOrName string) (*schema.VMRecoveryPointIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an vm by its UUID. If the vm does not exist, nil is returned.
func (c *VMRecoveryPointClient) GetByUUID(ctx context.Context, uuid string) (*schema.VMRecoveryPointIntent, error) {
	response := new(schema.VMRecoveryPointIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(vmRecoveryPointSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an vm by its name. If the vm does not exist, nil is returned.
func (c *VMRecoveryPointClient) GetByName(ctx context.Context, name string) (*schema.VMRecoveryPointIntent, error) {
	list, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("name==%s", name)})
	if len(list.Entities) == 0 {
		return nil, fmt.Errorf("RecoveryPoint not found: %s", name)
	}
	return list.Entities[0], err
}

// List returns a list of VMRecoveryPoints
func (c *VMRecoveryPointClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.VMRecoveryPointListIntent, error) {
	response := new(schema.VMRecoveryPointListIntent)
	err := c.client.requestHelper(ctx, vmRecoveryPointListPath, http.MethodPost, opts, response)
	return response, err
}

// All returns all VMRecoveryPoints
func (c *VMRecoveryPointClient) All(ctx context.Context) (*schema.VMRecoveryPointListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}

// Create creates a VMRecoveryPoint
func (c *VMRecoveryPointClient) Create(ctx context.Context, createRequest *schema.VMRecoveryPointRequest) (*schema.VMRecoveryPointIntent, error) {
	response := new(schema.VMRecoveryPointIntent)
	err := c.client.requestHelper(ctx, vmRecoveryPointBasePath, http.MethodPost, createRequest, response)
	return response, err
}

// Delete deletes a VMRecoveryPoint
func (c *VMRecoveryPointClient) Delete(ctx context.Context, s *schema.VMRecoveryPointIntent) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(vmRecoveryPointSinglePath, s.Metadata.UUID), http.MethodDelete, nil, nil)
}
