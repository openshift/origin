package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	availabilityZoneBasePath   = "/availability_zones"
	availabilityZoneListPath   = availabilityZoneBasePath + "/list"
	availabilityZoneSinglePath = availabilityZoneBasePath + "/%s"
)

// AvailabilityZoneClient is a client for the project API.
type AvailabilityZoneClient struct {
	client *Client
}

// Get retrieves an project by its ID if the input can be parsed as an integer, otherwise it
// retrieves an image by its name. If the image does not exist, nil is returned.
func (c *AvailabilityZoneClient) Get(ctx context.Context, idOrName string) (*schema.AvailabilityZoneIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an image by its UUID. If the image does not exist, nil is returned.
func (c *AvailabilityZoneClient) GetByUUID(ctx context.Context, uuid string) (*schema.AvailabilityZoneIntent, error) {
	req, err := c.client.NewV3PCRequest(ctx, "GET", fmt.Sprintf(availabilityZoneSinglePath, uuid), nil)
	if err != nil {
		return nil, err
	}

	availabilityZoneIntent := new(schema.AvailabilityZoneIntent)
	err = c.client.Do(req, &availabilityZoneIntent)
	if err != nil {
		return nil, err
	}
	return availabilityZoneIntent, nil

}

// GetByName retrieves an project by its name. If the project does not exist, nil is returned.
func (c *AvailabilityZoneClient) GetByName(ctx context.Context, name string) (*schema.AvailabilityZoneIntent, error) {
	list, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("name==%s", name)})
	if len(list.Entities) == 0 {
		return nil, fmt.Errorf("AvailabilityZone not found: %s", name)
	}
	return list.Entities[0], err
}

// List returns a list of projects for a specific page.
func (c *AvailabilityZoneClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.AvailabilityZoneListIntent, error) {
	response := new(schema.AvailabilityZoneListIntent)
	err := c.client.requestHelper(ctx, availabilityZoneListPath, http.MethodPost, opts, response)
	return response, err
}

// All returns all images.
func (c *AvailabilityZoneClient) All(ctx context.Context) (*schema.AvailabilityZoneListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}
