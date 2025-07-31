package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	hostBasePath   = "/hosts"
	hostListPath   = hostBasePath + "/list"
	hostSinglePath = hostBasePath + "/%s"
)

// HostClient is a client for the host API.
type HostClient struct {
	client *Client
}

// Get retrieves an host by its UUID if the input can be parsed as an uuid, otherwise it
// retrieves an host by its name
func (c *HostClient) Get(ctx context.Context, idOrName string) (*schema.HostIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves a host by its UUID
func (c *HostClient) GetByUUID(ctx context.Context, uuid string) (*schema.HostIntent, error) {
	response := new(schema.HostIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(hostSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an host by its name
func (c *HostClient) GetByName(ctx context.Context, name string) (*schema.HostIntent, error) {
	list, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("name==%s", name)})
	if err != nil {
		return nil, err
	}
	if len(list.Entities) == 0 {
		return nil, fmt.Errorf("host not found: %s", name)
	}
	return list.Entities[0], err
}

// List returns a list of hosts for a specific page.
func (c *HostClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.HostListIntent, error) {
	response := new(schema.HostListIntent)
	err := c.client.requestHelper(ctx, hostListPath, http.MethodPost, opts, response)
	return response, err
}

// All returns all hosts
func (c *HostClient) All(ctx context.Context) (*schema.HostListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}
