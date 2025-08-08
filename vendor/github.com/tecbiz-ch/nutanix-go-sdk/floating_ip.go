package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	floatingIPBasePath   = "/floating_ips"
	floatingIPListPath   = floatingIPBasePath + "/list"
	floatingIPSinglePath = floatingIPBasePath + "/%s"
)

// FloatingIPClient is a client for the vpc API.
type FloatingIPClient struct {
	client *Client
}

// Get retrieves an FlotatingIp by its UUID if the input can be parsed as an uuid, otherwise it
// retrieves a FlotatingIp by its name
func (c *FloatingIPClient) Get(ctx context.Context, idOrName string) (*schema.FloatingIPIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an FlotatingIp by its UUID
func (c *FloatingIPClient) GetByUUID(ctx context.Context, uuid string) (*schema.FloatingIPIntent, error) {
	response := new(schema.FloatingIPIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(floatingIPSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an FlotatingIp by its name
func (c *FloatingIPClient) GetByName(ctx context.Context, name string) (*schema.FloatingIPIntent, error) {
	list, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("floating_ip==%s", name)})
	if err != nil {
		return nil, err
	}
	if len(list.Entities) == 0 {
		return nil, fmt.Errorf("floating_ip not found: %s", name)
	}
	return list.Entities[0], err
}

// List returns a list of FlotatingIp's
func (c *FloatingIPClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.FloatingIPListIntent, error) {
	response := new(schema.FloatingIPListIntent)
	err := c.client.requestHelper(ctx, floatingIPListPath, http.MethodPost, opts, response)
	return response, err

}

// All returns all FlotatingIp's
func (c *FloatingIPClient) All(ctx context.Context) (*schema.FloatingIPListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}

// Create creates a FlotatingIp
func (c *FloatingIPClient) Create(ctx context.Context, createRequest *schema.FloatingIPIntent) (*schema.FloatingIPIntent, error) {
	response := new(schema.FloatingIPIntent)
	err := c.client.requestHelper(ctx, floatingIPBasePath, http.MethodPost, createRequest, response)
	return response, err
}

// Update a FlotatingIp
func (c *FloatingIPClient) Update(ctx context.Context, fip *schema.FloatingIPIntent) (*schema.FloatingIPIntent, error) {
	fip.Status = nil
	response := new(schema.FloatingIPIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(floatingIPSinglePath, fip.Metadata.UUID), http.MethodPut, fip, response)
	return response, err
}

// Delete deletes a FlotatingIp
func (c *FloatingIPClient) Delete(ctx context.Context, uuid string) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(floatingIPSinglePath, uuid), http.MethodDelete, nil, nil)
}
