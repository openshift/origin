package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	taskBasePath   = "/tasks"
	taskListPath   = taskBasePath + "/list"
	taskSinglePath = taskBasePath + "/%s"
)

// TaskClient is a client for the subnet API.
type TaskClient struct {
	client *Client
}

// Get retrieves an project by its ID if the input can be parsed as an integer, otherwise it
// retrieves an image by its name. If the image does not exist, nil is returned.
func (c *TaskClient) Get(ctx context.Context, idOrName string) (*schema.Task, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an task by its UUID. If the image does not exist, nil is returned.
func (c *TaskClient) GetByUUID(ctx context.Context, uuid string) (*schema.Task, error) {
	response := new(schema.Task)
	err := c.client.requestHelper(ctx, fmt.Sprintf(taskSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an task by its name. If the project does not exist, nil is returned.
func (c *TaskClient) GetByName(ctx context.Context, name string) (*schema.Task, error) {
	list, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("operation_type==%s", name)})
	if err != nil {
		return nil, err
	}
	if len(list.Entities) == 0 {
		return nil, err
	}
	return list.Entities[0], err
}

// List returns a list of tasks for a specific page.
func (c *TaskClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.TaskListIntent, error) {
	response := new(schema.TaskListIntent)
	err := c.client.requestHelper(ctx, taskListPath, http.MethodPost, opts, response)
	return response, err
}

// All returns all tasks.
func (c *TaskClient) All(ctx context.Context) (*schema.TaskListIntent, error) {
	// now := time.Now().Add(-41000 * time.Hour)
	// nanos := now.UTC().UnixNano()
	// pp.Println(fmt.Sprintf("last_updated_time_usecs=gt=%d", nanos))
	// TODO: How to query the latest tasks based ob Status or Time created?
	return c.List(ctx, &schema.DSMetadata{
		SortAttribute: "start_time",
		SortOrder:     "ASCENDING",
		//Filter:        "status==SUCCEEDED",
		Length: utils.Int64Ptr(500),
	})
}

// Delete deletes a Task
func (c *TaskClient) Delete(ctx context.Context, s *schema.Task) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(taskSinglePath, *s.UUID), http.MethodDelete, nil, nil)
}
