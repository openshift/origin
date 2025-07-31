package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	routingPolicyBasePath   = "/routing_policies"
	routingPolicyListPath   = routingPolicyBasePath + "/list"
	routingPolicySinglePath = routingPolicyBasePath + "/%s"
)

// RoutingPolicyClient is a client for the vpc API.
type RoutingPolicyClient struct {
	client *Client
}

// GetByUUID retrieves an FlotatingIp by its UUID
func (c *RoutingPolicyClient) GetByUUID(ctx context.Context, uuid string) (*schema.RoutingPolicyIntent, error) {
	response := new(schema.RoutingPolicyIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(routingPolicySinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// List returns a list of FlotatingIp's
func (c *RoutingPolicyClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.RoutingPolicyListIntent, error) {
	response := new(schema.RoutingPolicyListIntent)
	err := c.client.requestHelper(ctx, routingPolicyListPath, http.MethodPost, opts, response)
	return response, err

}

// All returns all FlotatingIp's
func (c *RoutingPolicyClient) All(ctx context.Context) (*schema.RoutingPolicyListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}

// Create creates a FlotatingIp
func (c *RoutingPolicyClient) Create(ctx context.Context, createRequest *schema.RoutingPolicyIntent) (*schema.RoutingPolicyIntent, error) {
	response := new(schema.RoutingPolicyIntent)
	err := c.client.requestHelper(ctx, routingPolicyBasePath, http.MethodPost, createRequest, response)
	return response, err
}

// Update a FlotatingIp
func (c *RoutingPolicyClient) Update(ctx context.Context, r *schema.RoutingPolicyIntent) (*schema.RoutingPolicyIntent, error) {
	r.Status = nil
	response := new(schema.RoutingPolicyIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(routingPolicySinglePath, r.Metadata.UUID), http.MethodPut, r, response)
	return response, err
}

// Delete deletes a FlotatingIp
func (c *RoutingPolicyClient) Delete(ctx context.Context, uuid string) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(routingPolicySinglePath, uuid), http.MethodDelete, nil, nil)
}
