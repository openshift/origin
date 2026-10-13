package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	projectBasePath   = "/projects"
	projectListPath   = projectBasePath + "/list"
	projectSinglePath = projectBasePath + "/%s"
)

// ProjectClient is a client for the project API.
type ProjectClient struct {
	client *Client
}

// Get retrieves an project by its UUID if the input can be parsed as an uuid, otherwise it
// retrieves a project by its name
func (c *ProjectClient) Get(ctx context.Context, idOrName string) (*schema.ProjectIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an project by its UUID
func (c *ProjectClient) GetByUUID(ctx context.Context, uuid string) (*schema.ProjectIntent, error) {
	response := new(schema.ProjectIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(projectSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an project by its name
func (c *ProjectClient) GetByName(ctx context.Context, name string) (*schema.ProjectIntent, error) {
	list, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("name==%s", name)})
	if err != nil {
		return nil, err
	}
	if len(list.Entities) == 0 {
		return nil, fmt.Errorf("project not found: %s", name)
	}
	return list.Entities[0], err
}

// List returns a list of projects
func (c *ProjectClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.ProjectListIntent, error) {
	response := new(schema.ProjectListIntent)
	err := c.client.requestHelper(ctx, projectListPath, http.MethodPost, opts, response)
	return response, err

}

// All returns all projects
func (c *ProjectClient) All(ctx context.Context) (*schema.ProjectListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}

// Create creates a project
func (c *ProjectClient) Create(ctx context.Context, createRequest *schema.ProjectIntent) (*schema.ProjectIntent, error) {
	response := new(schema.ProjectIntent)
	err := c.client.requestHelper(ctx, projectBasePath, http.MethodPost, createRequest, response)
	return response, err
}

// Update a project
func (c *ProjectClient) Update(ctx context.Context, project *schema.ProjectIntent) (*schema.ProjectIntent, error) {
	project.Status = nil
	response := new(schema.ProjectIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(projectSinglePath, project.Metadata.UUID), http.MethodPut, project, response)
	return response, err
}

// Delete deletes a project
func (c *ProjectClient) Delete(ctx context.Context, uuid string) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(projectSinglePath, uuid), http.MethodDelete, nil, nil)
}
