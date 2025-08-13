package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	vpcBasePath   = "/vpcs"
	vpcListPath   = vpcBasePath + "/list"
	vpcSinglePath = vpcBasePath + "/%s"
)

// VpcClient is a client for the vpc API.
type VpcClient struct {
	client *Client
}

// Get retrieves an vpc by its UUID if the input can be parsed as an uuid, otherwise it
// retrieves a vpc by its name
func (c *VpcClient) Get(ctx context.Context, idOrName string) (*schema.VpcIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an vpc by its UUID
func (c *VpcClient) GetByUUID(ctx context.Context, uuid string) (*schema.VpcIntent, error) {
	response := new(schema.VpcIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(vpcSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an vpc by its name
func (c *VpcClient) GetByName(ctx context.Context, name string) (*schema.VpcIntent, error) {
	list, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("name==%s", name)})
	if err != nil {
		return nil, err
	}
	if len(list.Entities) == 0 {
		return nil, fmt.Errorf("vpc not found: %s", name)
	}
	return list.Entities[0], err
}

// List returns a list of vpc's
func (c *VpcClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.VpcListIntent, error) {
	response := new(schema.VpcListIntent)
	err := c.client.requestHelper(ctx, vpcListPath, http.MethodPost, opts, response)
	return response, err

}

// All returns all vpc's
func (c *VpcClient) All(ctx context.Context) (*schema.VpcListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}

// Create creates a vpc
func (c *VpcClient) Create(ctx context.Context, createRequest *schema.VpcIntent) (*schema.VpcIntent, error) {
	response := new(schema.VpcIntent)
	err := c.client.requestHelper(ctx, vpcBasePath, http.MethodPost, createRequest, response)
	return response, err
}

// Update a vpc
func (c *VpcClient) Update(ctx context.Context, vpc *schema.VpcIntent) (*schema.VpcIntent, error) {
	vpc.Status = nil
	response := new(schema.VpcIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(vpcSinglePath, vpc.Metadata.UUID), http.MethodPut, vpc, response)
	return response, err
}

// Delete deletes a vpc
func (c *VpcClient) Delete(ctx context.Context, uuid string) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(vpcSinglePath, uuid), http.MethodDelete, nil, nil)
}
